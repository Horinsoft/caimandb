// Package query holds SELECT-list parsing/evaluation that has no
// dependency on the Engine: classifying a SELECT entry into a plain
// field, a COUNT(field), or a two-operand arithmetic expression, and
// evaluating the latter two against a document's data map.
//
// The core Query/Filter/QueryResult/SortField types themselves remain
// in the root package (ops_find.go): they're threaded through dozens
// of Engine methods, and lifting them out would be a much larger,
// riskier change than this focused piece. See MIGRATION_NOTES.md.
//
// This file extends FIND's SELECT list beyond plain field names to cover
// two more shapes documented in the NQL reference:
//
//	SELECT title, COUNT(actors) as actors_count, year
//	SELECT title, duration_minutes, duration_minutes / 60 as hours
//	SELECT title, (rating * year) as weighted_score
//
// buildFindQuery still collects the raw comma-separated SELECT list
// exactly as before (see collectFieldList in cmd_find.go); this file
// classifies each entry into a plain field, a COUNT(field) aggregate, or
// a two-operand arithmetic expression, and evaluates the latter two
// against a document's Data so the result can be projected under its
// alias like any other field.
package query

import (
	"strconv"
	"strings"
)

type selectKind int

const (
	// SelectPlain is an ordinary field/path, e.g. "title" or
	// "directors.name" -- projected exactly as before this file existed.
	SelectPlain selectKind = iota
	// SelectCount is "COUNT(field) [AS alias]".
	SelectCount
	// SelectExprArith is "left OP right [AS alias]", optionally wrapped
	// in one pair of parentheses, where OP is one of + - * / and each
	// operand is a field path or a numeric literal.
	SelectExprArith
)

type SelectItem struct {
	Kind selectKind

	// Raw is the field/path text for SelectPlain.
	Raw string
	// Alias is the output key: the field itself for a plain entry with
	// no "AS", or the explicit/derived alias otherwise.
	Alias string

	// CountField is the argument of COUNT(...) for SelectCount.
	CountField string

	// Left/Right are the two operand texts for SelectExprArith --
	// either a field path or a numeric literal (see LeftIsNumber below).
	Left, Right   string
	LeftIsNumber  bool
	RightIsNumber bool
	LeftNum       float64
	RightNum      float64
	Op            byte // '+', '-', '*', or '/'
}

// ParseSelectItem classifies one raw SELECT entry (already split out of
// the comma-separated list by collectFieldList) into a plain field, a
// COUNT(...) aggregate, or a simple arithmetic expression. Anything that
// doesn't match either special shape falls back to SelectPlain, so every
// SELECT list that worked before this file existed keeps working
// identically.
func ParseSelectItem(raw string) SelectItem {
	field, alias := splitAsAlias(raw)
	trimmedField := strings.TrimSpace(field)
	upperField := strings.ToUpper(trimmedField)

	if strings.HasPrefix(upperField, "COUNT(") && strings.HasSuffix(trimmedField, ")") {
		inner := strings.TrimSpace(trimmedField[len("COUNT(") : len(trimmedField)-1])
		if alias == "" {
			alias = "count_" + sanitizeAlias(inner)
		}
		return SelectItem{Kind: SelectCount, CountField: inner, Alias: alias, Raw: raw}
	}

	if left, op, right, ok := splitArith(trimmedField); ok {
		item := SelectItem{Kind: SelectExprArith, Left: left, Right: right, Op: op, Raw: raw}
		if num, err := strconv.ParseFloat(left, 64); err == nil {
			item.LeftIsNumber, item.LeftNum = true, num
		}
		if num, err := strconv.ParseFloat(right, 64); err == nil {
			item.RightIsNumber, item.RightNum = true, num
		}
		if alias == "" {
			alias = sanitizeAlias(trimmedField)
		}
		item.Alias = alias
		return item
	}

	if alias != "" {
		return SelectItem{Kind: SelectPlain, Raw: trimmedField, Alias: alias}
	}
	return SelectItem{Kind: SelectPlain, Raw: trimmedField, Alias: trimmedField}
}

// splitAsAlias splits "expr AS alias" (case-insensitive) into the
// expression text and the alias name, using the *last* standalone "AS"
// token so the alias itself is never mistaken for one. Returns
// alias == "" when there's no AS clause.
func splitAsAlias(raw string) (expr, alias string) {
	fields := strings.Fields(raw)
	for idx := len(fields) - 1; idx > 0; idx-- {
		if strings.EqualFold(fields[idx-1], "AS") {
			return strings.Join(fields[:idx-1], " "), fields[idx]
		}
	}
	return strings.TrimSpace(raw), ""
}

// splitArith recognizes "left OP right", optionally wrapped in a single
// outer pair of parentheses, where OP is one of + - * /. This
// deliberately supports only one binary operation -- no precedence, no
// nested expressions -- which is enough for the documented "field / N AS
// alias" and "(a * b) AS alias" forms without a full expression parser.
func splitArith(expr string) (left string, op byte, right string, ok bool) {
	e := strings.TrimSpace(expr)
	if strings.HasPrefix(e, "(") && strings.HasSuffix(e, ")") {
		e = strings.TrimSpace(e[1 : len(e)-1])
	}
	tokens := strings.Fields(e)
	if len(tokens) != 3 || len(tokens[1]) != 1 {
		return "", 0, "", false
	}
	switch tokens[1][0] {
	case '+', '-', '*', '/':
		return tokens[0], tokens[1][0], tokens[2], true
	}
	return "", 0, "", false
}

// sanitizeAlias turns an expression's text into a safe map key to use as
// a default alias when the query didn't give one explicitly, e.g.
// "rating * year" -> "rating_year".
func sanitizeAlias(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
				b.WriteRune('_')
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// operandValue resolves one arithmetic operand against data: the literal
// number if it parsed as one, otherwise the numeric value of that field
// path (0 if missing/non-numeric, matching toF64's existing behavior
// elsewhere in the engine).
func operandValue(isNum bool, num float64, name string, data map[string]any) float64 {
	if isNum {
		return num
	}
	return getNestedF64(data, name)
}

// EvalArith computes a SelectExprArith item's value for one document.
// Division by zero returns nil rather than +Inf/NaN, since those don't
// round-trip cleanly through JSON output.
func EvalArith(item SelectItem, data map[string]any) any {
	l := operandValue(item.LeftIsNumber, item.LeftNum, item.Left, data)
	r := operandValue(item.RightIsNumber, item.RightNum, item.Right, data)
	switch item.Op {
	case '+':
		return l + r
	case '-':
		return l - r
	case '*':
		return l * r
	case '/':
		if r == 0 {
			return nil
		}
		return l / r
	}
	return nil
}

// CountValue computes COUNT(field) for one document: the number of
// elements when field resolves to an array (a plain array field, or a
// RELATE alias resolved/placeholder array), 1 if it resolves to any
// other non-nil value, 0 when the field is absent/null.
func CountValue(field string, data map[string]any) float64 {
	v := getNested(data, field)
	if arr, ok := v.([]any); ok {
		return float64(len(arr))
	}
	if v == nil {
		return 0
	}
	return 1
}

// ApplySelectExprs returns a copy of data with every non-plain item in
// items evaluated and written in under its alias (COUNT/arithmetic
// results); plain items are left for project()/resolveRelations to
// handle exactly as before. Returns data unchanged (no copy) when items
// has nothing to compute, so the common case of an all-plain SELECT list
// costs nothing extra.
func ApplySelectExprs(data map[string]any, items []SelectItem) map[string]any {
	hasComputed := false
	for _, it := range items {
		if it.Kind != SelectPlain {
			hasComputed = true
			break
		}
	}
	if !hasComputed {
		return data
	}

	out := make(map[string]any, len(data)+len(items))
	for k, v := range data {
		out[k] = v
	}
	for _, it := range items {
		switch it.Kind {
		case SelectCount:
			out[it.Alias] = CountValue(it.CountField, out)
		case SelectExprArith:
			out[it.Alias] = EvalArith(it, out)
		}
	}
	return out
}

// SelectProjectFields returns the plain field-list form of items, for
// callers (project(), resolveRelations()) that still expect a []string
// of output field names: a plain item projects under its own path, and a
// computed item projects under its alias, which ApplySelectExprs will
// already have written into the document's Data by the time this list is
// used.
func SelectProjectFields(items []SelectItem) []string {
	fields := make([]string, 0, len(items))
	for _, it := range items {
		if it.Kind == SelectPlain {
			fields = append(fields, it.Raw)
		} else {
			fields = append(fields, it.Alias)
		}
	}
	return fields
}
