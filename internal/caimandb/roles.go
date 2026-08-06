// roles.go define el catálogo de roles de usuario de CaimanDB y las
// reglas asociadas (normalización de alias antiguos, validación, y la
// regla de "un solo ADMIN por servidor").
package caimandb

import "strings"

// Roles soportados por CREATE USER ... ROLE <...>
const (
	RoleAdmin     = "ADMIN"     // dueño del servidor: único, se define al arrancar (o vía CREATE SERVER)
	RoleSubadmin  = "SUBADMIN"  // administración delegada (usuarios, bases, bloques) sin ser el dueño
	RoleDeveloper = "DEVELOPER" // lectura/escritura de datos y bloques
	RoleUser      = "USER"      // acceso limitado, normalmente solo lectura/escritura de datos
)

// validRoles enumera los roles válidos para CREATE USER, en el orden en
// que deben mostrarse (SHOW ROLES).
var validRoles = []string{RoleAdmin, RoleSubadmin, RoleDeveloper, RoleUser}

// roleDescriptions documenta cada rol para SHOW ROLES / HELP.
var roleDescriptions = map[string]string{
	RoleAdmin:     "Dueño del servidor. Solo puede existir uno por servidor. Acceso total.",
	RoleSubadmin:  "Administración delegada: usuarios, bases y bloques, sin poder crear otro ADMIN.",
	RoleDeveloper: "Lectura y escritura de datos, bases y bloques asignados.",
	RoleUser:      "Acceso limitado a las bases/bloques asignados.",
}

// normalizeRole convierte un token de rol (en cualquier mayúscula/minúscula)
// a su forma canónica, incluyendo los alias heredados de versiones
// anteriores de CaimanDB ("admin", "readwrite", "readonly") para no romper
// instalaciones existentes.
func normalizeRole(role string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case RoleAdmin, "ADMINISTRATOR":
		return RoleAdmin, true
	case RoleSubadmin, "SUB-ADMIN", "SUB_ADMIN":
		return RoleSubadmin, true
	case RoleDeveloper, "DEV", "READWRITE":
		return RoleDeveloper, true
	case RoleUser, "READONLY":
		return RoleUser, true
	default:
		return "", false
	}
}

// IsAdminRole reporta si role (ya normalizado) es el rol de dueño del
// servidor.
func IsAdminRole(role string) bool {
	return strings.ToUpper(role) == RoleAdmin
}

// ShowRoles devuelve el catálogo de roles para el comando "SHOW ROLES".
func (e *Engine) ShowRoles() []map[string]any {
	out := make([]map[string]any, 0, len(validRoles))
	for _, r := range validRoles {
		out = append(out, map[string]any{
			"role":        r,
			"description": roleDescriptions[r],
		})
	}
	return out
}
