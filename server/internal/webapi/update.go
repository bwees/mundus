package webapi

import (
	"io"
	"net/http"

	"github.com/go-fuego/fuego"

	"github.com/bwees/mundus/server/internal/update"
)

func registerUpdate(api *fuego.Server, d Deps) {
	fuego.Get(api, "/update", func(ctx fuego.ContextNoBody) (update.Status, error) {
		return d.Update.Status(), nil
	}, fuego.OptionOperationID("getUpdate"))

	fuego.Post(api, "/update/check", func(ctx fuego.ContextNoBody) (update.Status, error) {
		st, err := d.Update.Check(ctx.Context())
		return st, err
	}, fuego.OptionOperationID("checkUpdate"))

	fuego.Post(api, "/update/apply", func(ctx fuego.ContextNoBody) (OK, error) {
		if err := d.Update.Apply(ctx.Context()); err != nil {
			return OK{}, err
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("applyUpdate"))

	// Manual upload: multipart form with "binary" (the server binary) and
	// optional "web" (web.tar.gz). Raw handler — not part of the typed API.
	upload := func(w http.ResponseWriter, r *http.Request) {
		if d.Update == nil {
			http.Error(w, "updates unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		bin, _, err := r.FormFile("binary")
		if err != nil {
			http.Error(w, "missing 'binary' file", http.StatusBadRequest)
			return
		}
		defer bin.Close()
		binBytes, err := io.ReadAll(bin)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var webBytes []byte
		if web, _, err := r.FormFile("web"); err == nil {
			defer web.Close()
			webBytes, _ = io.ReadAll(web)
		}
		if err := d.Update.ApplyUpload(binBytes, webBytes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
	fuego.Handle(api, "/update/upload", http.HandlerFunc(upload), fuego.OptionHide())
}
