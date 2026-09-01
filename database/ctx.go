package database

import "context"

// ctx devuelve un contexto de fondo compartido para las consultas.
func ctx() context.Context { return context.Background() }
