package handlers

import (
	"net/http"

	"seduhpoin/internal/format"
)

// dashboard menampilkan ringkasan operasional kedai.
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	hariIni := format.HariIni()
	data, err := a.Store.AmbilDashboard(r.Context(), hariIni)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat dashboard", err.Error())
		return
	}
	a.render(w, "dashboard/list.html", View{
		Title: "Dashboard",
		Menu:  "dashboard",
		Data:  data,
	})
}
