// MySYS — Sistema de Facturación (versión Go)
//
// Equivalente a main.py: arranca el servidor principal (puerto 8000) y la
// app de registro (puerto 8001) en el mismo proceso, con apagado limpio
// ante Ctrl+C. Compila a un único ejecutable (no requiere Python ni py2exe).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Limpiar pantalla (equivalente a cls/clear)
	fmt.Print("\033[H\033[2J")

	initTemplates()
	initDBs()

	// Asegurar carpetas de subidas
	for _, dir := range []string{CarpetaImagenes, CarpetaPerfilImg, CarpetaArchivos} {
		_ = os.MkdirAll(dir, 0o755)
	}

	servidor := &http.Server{
		Addr:    "0.0.0.0:8000",
		Handler: nuevoServidor(),
	}
	registro := &http.Server{
		Addr:    "127.0.0.1:8001",
		Handler: nuevoRegistro(),
	}

	// Limpieza periódica de sesiones del escáner (cada hora, como en Python)
	ctxLimpieza, cancelarLimpieza := context.WithCancel(context.Background())
	defer cancelarLimpieza()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				limpiarSesionesAntiguas()
			case <-ctxLimpieza.Done():
				return
			}
		}
	}()

	fmt.Println("Servidor iniciado en http://localhost:8000")
	fmt.Printf("Puedes visitar el servidor en http://%s:8000\n", obtenerIPLocal())
	fmt.Println("----------------------------------------")
	fmt.Println("Si no existe un usuario usa el registro para iniciar con un usuario administrador.")
	fmt.Println("Registro iniciado en http://localhost:8001")
	fmt.Println("----------------------------------------")

	errores := make(chan error, 2)
	go func() {
		if err := servidor.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errores <- fmt.Errorf("servidor principal: %w", err)
		}
	}()
	go func() {
		if err := registro.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errores <- fmt.Errorf("servidor de registro: %w", err)
		}
	}()

	// Esperar Ctrl+C (SIGINT) o SIGTERM
	senales := make(chan os.Signal, 1)
	signal.Notify(senales, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errores:
		fmt.Printf("Error en el servidor: %v\n", err)
	case <-senales:
		fmt.Println("Deteniendo servidores...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = servidor.Shutdown(ctx)
	_ = registro.Shutdown(ctx)
}
