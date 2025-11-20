// Package server provides functions to set a webhook and start a server to process incoming requests.
package server

import (
	"context"
	"github.com/kozalosev/goSadTgBot/logconst"
	log "log/slog"
	"net/http"
	"os"
	"time"
)

// Start a server listening on the specified port.
func Start(port string) *http.Server {
	srv := &http.Server{Addr: ":" + port}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Error("Couldn't start the server",
				logconst.FieldFunc, "startServer",
				logconst.FieldCalledObject, "Server",
				logconst.FieldCalledMethod, "ListenAndServe",
				logconst.FieldError, err)
			os.Exit(1)
		}
	}()
	return srv
}

// StopListeningForIncomingRequests stops the server.
func StopListeningForIncomingRequests(srv *http.Server) {
	ctx, c := context.WithTimeout(context.Background(), time.Minute)
	defer c()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Error while shutting down the server",
			logconst.FieldFunc, "stopListeningForIncomingRequests",
			logconst.FieldCalledObject, "Server",
			logconst.FieldCalledMethod, "Shutdown",
			logconst.FieldError, err)
	}
}
