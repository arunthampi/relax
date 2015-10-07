package healthcheck

import (
	"fmt"
	"net/http"

	log "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/facebookgo/grace/gracehttp"
)

type HealthCheckServer struct {
}

func (hs *HealthCheckServer) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "relax alive")
}

func (hs *HealthCheckServer) Start(host string, port uint16) {
	svr := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: hs}
	if err := gracehttp.Serve(svr); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("relax: cannot start healthcheck server")
	} else {
		log.Info("relax: started healthcheck server on: %s", fmt.Sprintf("%s:%d", host, port))
	}

	log.Info("relax: healthcheck server stopped")
}
