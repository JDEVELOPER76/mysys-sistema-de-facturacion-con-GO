package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"mysys/database"
)

// ==========================================
//   APP DE REGISTRO (puerto 8001)
//   Equivalente a register.py: permite crear el primer usuario administrador.
// ==========================================

func nuevoRegistro() http.Handler {
	r := chi.NewRouter()
	r.Use(recover500)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		renderError(w, req, http.StatusNotFound, "")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		renderError(w, req, http.StatusMethodNotAllowed, "Método no permitido")
	})

	// Archivos estáticos (comparte la carpeta static de la app principal)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	r.Get("/", indexRegistro)
	r.Get("/cuenta-registrada-con-exito", cuentaRegistradaConExito)
	r.Post("/registro", registrarUsuario)

	return r
}

func indexRegistro(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	render(w, r, "register.html", map[string]any{
		"error":   q.Get("error"),
		"success": q.Get("success"),
	})
}

func cuentaRegistradaConExito(w http.ResponseWriter, r *http.Request) {
	render(w, r, "registro_exitoso.html", map[string]any{})
}

// registrarUsuario procesa el formulario de registro inicial (OAuth2 form: username/password).
func registrarUsuario(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderError(w, r, http.StatusBadRequest, "Formulario inválido")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if loginDB.VerificarUsuario(username, password) {
		jsonError(w, http.StatusBadRequest, "Ya existe un usuario registrado.")
		return
	}
	loginDB.AgregarUsuario(database.User{Username: username, Password: password, Tipo: "admin"})
	redirigir(w, r, "/cuenta-registrada-con-exito")
}
