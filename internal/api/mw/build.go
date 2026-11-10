package mw

import "net/http"

const BuildHeader = "X-Tuby-Build"

func RequireCurrentBuild(buildID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if staleClientRequest(request, buildID) {
				writer.Header().Set("Cache-Control", "no-store")
				writer.Header().Set("HX-Refresh", "true")
				writer.WriteHeader(http.StatusConflict)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func staleClientRequest(request *http.Request, buildID string) bool {
	clientBuild := request.Header.Get(BuildHeader)
	isVersionedClient := request.Header.Get("HX-Request") == "true" || clientBuild != ""
	return buildID != "" && isVersionedClient && clientBuild != buildID
}
