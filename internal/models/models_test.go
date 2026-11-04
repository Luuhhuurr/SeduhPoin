package models

import (
	"testing"
	"time"
)

// promoDasar membuat promo yang berlaku sepanjang hari di semua cabang.
//
// Objek ini adalah "kasus normal": aktif, masa berlaku 1 tahun, tanpa jam
// dan tanpa pembatasan cabang. Setiap test di bawahnya mengubah satu aspek
// saja supaya penyebab kegagalan jelas ketika test gagal.
func promoDasar() Promo {
	return Promo{
		ID:         1,
		Kode:       "PROMO-uji",
		Nama:       "Promo Uji",
		Jenis:      JenisDiskonProduk,
		Aktif:      true,
		TglMulai:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TglSelesai: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}
}

// TestBerlakuPadaMasaBerlaku menguji promo hanya berlaku di antara tanggal awal dan akhir.
//
// Membuktikan aturan BR: promo tidak boleh dipakai sebelum tgl_mulai maupun
// sesudah tgl_selesai.
func TestBerlakuPadaMasaBerlaku(t *testing.T) {
	p := promoDasar()

	sebelum := time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC)
	sesudah := time.Date(2027, 1, 1, 10, 0, 0, 0, time.UTC)
	tengah := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	if p.BerlakuPada(sebelum, 1) {
		t.Error("promo seharusnya tidak berlaku sebelum tgl_mulai")
	}
	if p.BerlakuPada(sesudah, 1) {
		t.Error("promo seharusnya tidak berlaku setelah tgl_selesai")
	}
	if !p.BerlakuPada(tengah, 1) {
		t.Error("promo seharusnya berlaku di antara tgl_mulai dan tgl_selesai")
	}
}

// TestBerlakuPadaJam menguji pembatasan jam (kosong = sepanjang hari).
//
// Membuktikan aturan BR: jam promo bersifat opsional; bila diisi, promo hanya
// berlaku dalam rentang jam tersebut dan jam_selesai masih termasuk berlaku.
func TestBerlakuPadaJam(t *testing.T) {
	p := promoDasar()
	p.JamMulai = "06:00:00"
	p.JamSelesai = "11:00:00"

	diDalam := time.Date(2026, 6, 15, 8, 30, 0, 0, time.UTC)
	tepatBatas := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC)
	diLuar := time.Date(2026, 6, 15, 13, 0, 0, 0, time.UTC)

	if !p.BerlakuPada(diDalam, 1) {
		t.Error("promo seharusnya berlaku pada jam 08:30")
	}
	if !p.BerlakuPada(tepatBatas, 1) {
		t.Error("promo seharusnya masih berlaku tepat pada jam_selesai")
	}
	if p.BerlakuPada(diLuar, 1) {
		t.Error("promo seharusnya tidak berlaku pada jam 13:00")
	}

	// Jam kosong berarti promo berlaku sepanjang hari.
	p.JamMulai = ""
	p.JamSelesai = ""
	if !p.BerlakuPada(diLuar, 1) {
		t.Error("promo tanpa jam seharusnya berlaku sepanjang hari")
	}
}

// TestBerlakuPadaCabang menguji pembatasan cabang (kosong = semua cabang).
//
// Membuktikan aturan BR: promo dapat dibatasi untuk cabang tertentu saja,
// dan daftar cabang kosong berarti berlaku untuk seluruh cabang.
func TestBerlakuPadaCabang(t *testing.T) {
	p := promoDasar()
	waktu := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	p.CabangIDs = []int64{1, 2}
	if !p.BerlakuPada(waktu, 1) {
		t.Error("promo seharusnya berlaku di cabang 1")
	}
	if p.BerlakuPada(waktu, 3) {
		t.Error("promo seharusnya tidak berlaku di cabang 3")
	}

	// Cabang kosong berarti berlaku untuk semua cabang.
	p.CabangIDs = nil
	if !p.BerlakuPada(waktu, 3) {
		t.Error("promo tanpa pembatasan cabang seharusnya berlaku di semua cabang")
	}
}

// TestBerlakuPadaStatusAktif menguji promo nonaktif tidak dapat dipakai.
//
// Membuktikan aturan BR: menonaktifkan promo langsung menonaktifkannya juga
// pada form transaksi, tanpa perlu menghapus datanya.
func TestBerlakuPadaStatusAktif(t *testing.T) {
	p := promoDasar()
	waktu := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	if !p.BerlakuPada(waktu, 1) {
		t.Error("promo aktif seharusnya berlaku")
	}
	p.Aktif = false
	if p.BerlakuPada(waktu, 1) {
		t.Error("promo nonaktif tidak boleh berlaku")
	}
}
