package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"seduhpoin/internal/auth"
	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
	"seduhpoin/internal/store"
)

// Peran yang dipakai untuk mengatur akses di seluruh aplikasi.
//
// "STAFF" bukan peran yang tersimpan di database, melainkan gabungan
// ADMIN dan KASIR: dua-duanya boleh memakai menu operasional (transaksi,
// riwayat, dan membaca data master).
const (
	peranStaff  = "STAFF"
	peranAdmin  = models.RoleAdmin
	peranMember = models.RoleMember
)

// Masa berlaku sesi. Cukup untuk satu shift kasir; setelahnya pengguna
// harus login lagi.
const masaSesi = 8 * time.Hour

// NamaCookieSesi adalah nama cookie penanda sesi login.
const NamaCookieSesi = "seduhpoin_sesi"

// Identitas adalah identitas pengguna yang sedang login.
//
// Diisi oleh middleware denganSesi dan disimpan di context request, sehingga
// handler tidak perlu membaca cookie lagi. Nilai nol (Masuk = false)
// berarti belum login.
type Identitas struct {
	Masuk           bool
	Peran           string // ADMIN / KASIR / MEMBER
	Nama            string
	Username        string // staf: username; member: nomor HP
	PenggunaID      int64
	MemberID        int64
	Token           string // token mentah cookie (dipakai saat logout)
	HarusGantiSandi bool
}

// Boleh melapor apakah peran saat ini boleh mengakses fitur dengan peran
// tertentu.
//
// Nilai peran yang bisa dipakai:
//
//	"STAFF"  -> ADMIN atau KASIR
//	"ADMIN"  -> hanya ADMIN
//	"MEMBER" -> hanya MEMBER
//
// Peran kosong ("") berarti apa saja, asal sudah login.
//
// Catatan: "STAFF" dipakai untuk hal yang boleh dilakukan keduanya (misalnya
// memproses transaksi atau membaca katalog produk). Aksi yang mengubah data
// master memakai "ADMIN", karena pengelolaan master data adalah wewenang
// pemilik, bukan kasir.
func (i Identitas) Boleh(peran string) bool {
	if !i.Masuk {
		return false
	}
	if peran == "" {
		return true
	}
	if i.Peran == peran {
		return true
	}
	if peran == peranStaff {
		return i.Peran == models.RoleAdmin || i.Peran == models.RoleKasir
	}
	return false
}

// LabelPeran mengubah kode peran menjadi teks yang enak dibaca.
func LabelPeran(peran string) string {
	switch peran {
	case models.RoleAdmin:
		return "Administrator"
	case models.RoleKasir:
		return "Kasir"
	case models.RoleMember:
		return "Member"
	default:
		return "-"
	}
}

// tipeKunciContext adalah tipe kunci khusus untuk menyimpan Identitas di context.
type tipeKunciContext struct{}

// kunciIdentitas adalah kunci context tempat Identitas disimpan.
var kunciIdentitas = tipeKunciContext{}

// registerAuth mendaftarkan rute login dan logout.
//
// Rute ini bersifat publik (tidak butuh sesi) kecuali /logout yang memang
// hanya berguna kalau sudah login.
func (a *App) registerAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", a.withDB(a.halamanLogin))
	mux.HandleFunc("POST /login", a.withDB(a.prosesLogin))
	mux.HandleFunc("POST /logout", a.withDB(a.logout))
}

// denganSesi membaca cookie sesi, mencari sesi di database, dan menaruh
// hasilnya di context request.
//
// Bila tidak ada sesi valid, handler tetap dijalankan dengan Identitas.Masuk
// = false, sehingga halaman publik (login) tetap bisa dibuka.
func (a *App) denganSesi(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := Identitas{}

		cookie, err := r.Cookie(NamaCookieSesi)
		if err == nil && cookie.Value != "" {
			hash := auth.HashToken(cookie.Value)
			hasil, err := a.Store.AmbilSesi(r.Context(), hash, format.WIB())
			switch {
			case err == nil:
				id = identitasDariSesi(hasil, cookie.Value)
			case err == store.ErrSesiTidakValid:
				// Sesi sudah tidak berlaku (kedaluwarsa, akun dinonaktifkan,
				// atau member dihapus): cookie dibersihkan supaya browser
				// berhenti mengirimnya.
				a.hapusCookieSesi(w)
			default:
				// Error lain (misalnya database sedang bermasalah) tidak
				// boleh membuat aplikasi berhenti; perlakukan saja sebagai
				// belum login.
				a.hapusCookieSesi(w)
			}
		}

		ctx := context.WithValue(r.Context(), kunciIdentitas, id)
		next(w, r.WithContext(ctx))
	}
}

// wajibLogin menolak request yang belum login dengan mengarahkan ke
// halaman login.
func (a *App) wajibLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !identitasDari(r).Masuk {
			redirectKeLogin(w, r)
			return
		}
		next(w, r)
	}
}

// wajibPeran menolak request yang perannya tidak sesuai.
//
// Belum login -> diarahkan ke halaman login. Sudah login tapi peran salah ->
// halaman "Akses ditolak". Menu di sidebar bukan pengaman; pengaman yang
// sesungguhnya ada di sini, di sisi server.
func (a *App) wajibPeran(peran string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := identitasDari(r)
		if !id.Masuk {
			redirectKeLogin(w, r)
			return
		}
		// Akun yang masih memakai kata sandi awal hanya boleh membuka
		// halaman ganti sandi. Semua halaman lain ditahan sampai sandi
		// digantikan, sehingga akun demo tidak pernah dipakai dengan sandi
		// yang everybody tahu.
		if id.HarusGantiSandi && !bolehLewatiGantiSandi(r) {
			http.Redirect(w, r, "/akun/sandi?saya=1", http.StatusSeeOther)
			return
		}
		if !id.Boleh(peran) {
			pesanError(w, http.StatusForbidden, "Akses ditolak",
				"Halaman ini hanya untuk peran "+LabelPeran(peran)+". Anda masuk sebagai "+LabelPeran(id.Peran)+".")
			return
		}
		next(w, r)
	}
}

// bolehLewatiGantiSandi melapor apakah request ini boleh proceeded walau
// akun masih harus mengganti kata sandi.
//
// Yang boleh: halaman ganti sandi itu sendiri, dan logout (supaya pengguna
// bisa keluar tanpa harus mengganti sandi lebih dulu).
func bolehLewatiGantiSandi(r *http.Request) bool {
	switch r.URL.Path {
	case "/akun/sandi", "/logout":
		return true
	}
	return false
}

// akses menggabungkan pemeriksaan database, pembacaan sesi, dan pemeriksaan
// peran menjadi satu middleware.
//
// Urutannya penting: withDB paling luar supaya error koneksi database tetap
// menampilkan halaman 503 yang rapi, dan pembacaan sesi di tengah supaya
// pemeriksaan peran selalu punya data identitas.
func (a *App) akses(peran string, next http.HandlerFunc) http.HandlerFunc {
	return a.withDB(a.denganSesi(a.wajibPeran(peran, next)))
}

// identitasDari membaca identitas yang disimpan di context request.
func identitasDari(r *http.Request) Identitas {
	if r == nil {
		return Identitas{}
	}
	if id, ok := r.Context().Value(kunciIdentitas).(Identitas); ok {
		return id
	}
	return Identitas{}
}

// identitasDariSesi menyusun Identitas dari baris sesi yang valid.
func identitasDariSesi(hasil store.SesiAktif, token string) Identitas {
	id := Identitas{
		Masuk: true,
		Peran: hasil.Sesi.Peran,
		Nama:  hasil.Pengguna.Nama,
		Token: token,
	}
	if hasil.Sesi.PenggunaID != nil {
		id.PenggunaID = *hasil.Sesi.PenggunaID
		id.Username = hasil.Pengguna.Username
		id.Nama = hasil.Pengguna.Nama
		id.HarusGantiSandi = hasil.Pengguna.HarusGantiSandi
	}
	if hasil.Sesi.MemberID != nil {
		id.MemberID = *hasil.Sesi.MemberID
		id.Username = hasil.Member.NoHP
		id.Nama = hasil.Member.Nama
	}
	return id
}

// buatSesi membuat baris sesi, menulis cookie, dan mengembalikan token.
//
// Token yang dikirim ke browser adalah token acak; yang disimpan di database
// hanya SHA-256 dari token itu (lihat auth.HashToken).
func (a *App) buatSesi(w http.ResponseWriter, r *http.Request, jenis string,
	penggunaID *int64, memberID *int64, peran, nama string) error {

	token, err := auth.TokenSesi()
	if err != nil {
		return err
	}
	sekarang := format.WIB()
	sesi := models.Sesi{
		TokenHash:     auth.HashToken(token),
		Jenis:         jenis,
		PenggunaID:    penggunaID,
		MemberID:      memberID,
		Peran:         peran,
		DibuatAt:      sekarang,
		KedaluwarsaAt: sekarang.Add(masaSesi),
	}
	if err := a.Store.BuatSesi(r.Context(), sesi); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:  NamaCookieSesi,
		Value: token,
		Path:  "/",
		// HttpOnly membuat cookie tidak bisa dibaca JavaScript, sehingga
		// cookie tidak bisa dicuri lewat serangan XSS.
		HttpOnly: true,
		// SameSite=Lax mencegah cookie ikut dikirim pada POST dari situs
		// lain, sehingga menjadi perlindungan dasar terhadap CSRF.
		SameSite: http.SameSiteLaxMode,
		// Secure hanya untuk koneksi HTTPS. Demo berjalan di
		// http://localhost, dan cookie bertanda Secure akan ditolak browser
		// di sana sehingga login selalu berputar balik.
		Secure: r.TLS != nil,
		MaxAge: int(masaSesi.Seconds()),
	})
	return nil
}

// hapusCookieSesi menghapus cookie sesi di browser.
func (a *App) hapusCookieSesi(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     NamaCookieSesi,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// hapusSesi menghapus sesi di database sekaligus membersihkan cookie.
//
// Menghapus baris sesi membuat token langsung tidak berlaku: inilah yang
// membuat logout bersifat instan, bukan sekadar menghapus cookie di browser.
func (a *App) hapusSesi(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(NamaCookieSesi); err == nil && cookie.Value != "" {
		_ = a.Store.HapusSesi(r.Context(), auth.HashToken(cookie.Value))
	}
	a.hapusCookieSesi(w)
}

// halamanLogin menampilkan form login.
//
// Kalau pengguna sudah login, halaman ini langsung mengarahkan ke beranda
// sesuai perannya supaya tidak ada dua sesi untuk satu browser.
func (a *App) halamanLogin(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)
	if id.Masuk {
		http.Redirect(w, r, berandaUntuk(id.Peran), http.StatusSeeOther)
		return
	}

	a.renderMandiri(w, "templates/auth/login.html", "login", map[string]any{
		"Error": ambilError(r),
		"Next":  r.URL.Query().Get("next"),
	})
}

// prosesLogin memproses POST dari form login.
//
// Satu form dipakai untuk staf dan member; field "jenis" menentukan akun
// mana yang dicari. Pesan kegagalan selalu sama supaya tidak bisa dipakai
// menebak username atau nomor HP mana yang terdaftar.
func (a *App) prosesLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	jenis := r.FormValue("jenis")
	identitas := strings.TrimSpace(r.FormValue("identitas"))
	sandi := r.FormValue("sandi")

	const pesanGagal = "Username/nomor HP atau kata sandi salah."

	if identitas == "" || sandi == "" {
		a.renderLoginGagal(w, r, "Username/nomor HP dan kata sandi wajib diisi.")
		return
	}

	var peran string
	var nama string
	var penggunaID *int64
	var memberID *int64
	var harusGantiSandi bool

	switch jenis {
	case models.JenisMember:
		member, hash, err := a.Store.CariMemberAkunByNoHP(r.Context(), identitas)
		if err != nil || !auth.CheckPassword(hash, sandi) {
			a.renderLoginGagal(w, r, pesanGagal)
			return
		}
		peran = models.RoleMember
		nama = member.Nama
		id := member.ID
		memberID = &id
	default:
		pengguna, err := a.Store.AmbilPenggunaByUsername(r.Context(), strings.ToLower(identitas))
		if err != nil || !auth.CheckPassword(pengguna.PasswordHash, sandi) {
			a.renderLoginGagal(w, r, pesanGagal)
			return
		}
		peran = pengguna.Role
		nama = pengguna.Nama
		id := pengguna.ID
		penggunaID = &id
		harusGantiSandi = pengguna.HarusGantiSandi
	}

	jenisSesi := models.JenisPengguna
	if memberID != nil {
		jenisSesi = models.JenisMember
	}
	if err := a.buatSesi(w, r, jenisSesi, penggunaID, memberID, peran, nama); err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal membuat sesi login", err.Error())
		return
	}

	// Sesi kedaluwarsa dibersihkan secara lazy di sini, tanpa cron.
	_ = a.Store.BersihkanSesiKedaluwarsa(r.Context(), format.WIB())

	if penggunaID != nil {
		_ = a.Store.CatatLoginPengguna(r.Context(), *penggunaID, format.WIB())
	}

	tujuan := tujuanSetelahLogin(r, peran)
	if harusGantiSandi {
		// Akun demo admin dipaksa mengganti kata sandi awal sebelum boleh
		// memakai aplikasi.
		tujuan = "/akun/sandi?saya=1"
	}
	http.Redirect(w, r, tujuan, http.StatusSeeOther)
}

// logout menghapus sesi lalu mengarahkan ke halaman login.
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	a.hapusSesi(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// renderLoginGagal menampilkan kembali form login dengan pesan error.
//
// Form di-render ulang (bukan redirect) supaya username yang sudah diketik
// tidak hilang dan pesan langsung terlihat.
func (a *App) renderLoginGagal(w http.ResponseWriter, r *http.Request, pesan string) {
	w.WriteHeader(http.StatusUnauthorized)
	a.renderMandiri(w, "templates/auth/login.html", "login", map[string]any{
		"Error": pesan,
		"Next":  r.URL.Query().Get("next"),
	})
}

// redirectKeLogin mengarahkan pengguna yang belum login ke halaman login,
// sambil mengingat halaman yang tadi dituju.
func redirectKeLogin(w http.ResponseWriter, r *http.Request) {
	tujuan := "/login"
	if next := r.URL.Query().Get("next"); next != "" {
		tujuan += "?next=" + url.QueryEscape(next)
	}
	http.Redirect(w, r, tujuan, http.StatusSeeOther)
}

// tujuanSetelahLogin menentukan halaman tujuan setelah login berhasil.
//
// Parameter ?next= dipakai kalau ada, tetapi hanya bila aman: harus dimulai
// dengan "/" dan bukan "//" (yang bisa mengarahkan ke situs lain).
func tujuanSetelahLogin(r *http.Request, peran string) string {
	if next := r.URL.Query().Get("next"); strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return berandaUntuk(peran)
}

// berandaUntuk mengembalikan halaman utama untuk sebuah peran.
//
// Admin mulai dari dashboard, kasir langsung ke form transaksi, dan member
// ke portal miliknya sendiri.
func berandaUntuk(peran string) string {
	switch peran {
	case models.RoleMember:
		return "/portal"
	case models.RoleKasir:
		return "/transaksi"
	default:
		return "/"
	}
}
