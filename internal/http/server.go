package http

import (
	"fmt"
	"mitoboat/internal/env"
	"net/http"
)

func Start() (*http.Server, error) {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", env.DefaultEnv.HttpPort),
	}

	return srv, nil
}
