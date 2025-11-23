package routes

import (
	"net/http"

	"github.com/terabiome/homonculus/internal/handler"
)

// Router wraps http.ServeMux and provides route setup
type Router struct {
	*http.ServeMux
}

// V1Handler returns a handler for v1 API routes
func (router *Router) V1Handler(vmHandler *handler.VirtualMachine, k3sHandler *handler.K3s, systemHandler *handler.System) http.Handler {
	mux := http.NewServeMux()

	// Setup virtual machine routes
	vmMux := http.NewServeMux()
	vmMux.HandleFunc("POST /create/cluster", vmHandler.CreateCluster)
	vmMux.HandleFunc("POST /delete/cluster", vmHandler.DeleteCluster)
	vmMux.HandleFunc("POST /start/cluster", vmHandler.StartCluster)
	vmMux.HandleFunc("GET /query/cluster", vmHandler.QueryCluster)
	vmMux.HandleFunc("POST /query/cluster", vmHandler.QueryCluster)
	vmMux.HandleFunc("POST /clone/cluster", vmHandler.CloneCluster)
	vmMux.HandleFunc("POST /format", vmHandler.FormatRequest)
	mux.Handle("/virtualmachine/", http.StripPrefix("/virtualmachine", vmMux))

	// Setup K3s routes
	k3sMux := http.NewServeMux()
	k3sMux.HandleFunc("POST /generate-token", k3sHandler.GenerateToken)
	k3sMux.HandleFunc("POST /bootstrap/master", k3sHandler.BootstrapMaster)
	k3sMux.HandleFunc("POST /bootstrap/worker", k3sHandler.BootstrapWorker)
	mux.Handle("/k3s/", http.StripPrefix("/k3s", k3sMux))

	// Setup system routes
	systemMux := http.NewServeMux()
	systemMux.HandleFunc("GET /cpu-topology", systemHandler.CPUTopology)
	mux.Handle("/system/", http.StripPrefix("/system", systemMux))

	return mux
}

// SetupMux creates and configures the main router
func SetupMux(vmHandler *handler.VirtualMachine, k3sHandler *handler.K3s, systemHandler *handler.System) *Router {
	router := Router{http.NewServeMux()}

	router.ServeMux.Handle("/api/v1/", http.StripPrefix("/api/v1", router.V1Handler(vmHandler, k3sHandler, systemHandler)))

	router.ServeMux.HandleFunc("/heartbeat", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte("i have not exploded"))
	})

	return &router
}
