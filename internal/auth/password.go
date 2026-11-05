// Package auth berisi primitif autentikasi SeduhPoin: hashing kata sandi
// (bcrypt) dan pembuatan token sesi acak.
//
// Package ini SENGAJA tidak menyentuh database; seluruh query ada di
// internal/store. Tujuannya agar aturan keamanan (misalnya format hash dan
// panjang token) berada di satu tempat yang mudah diuji.
//
// Aturan main proyek: tanpa framework web eksternal dan tanpa ORM, hanya
// library standar Go. bcrypt diambil dari golang.org/x/crypto karena
// pustaka standar belum menyediakan hashing password yang aman.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BiayaBCrypt adalah biaya kerja bcrypt. Nilai bawaan (10) cukup aman
	// untuk aplikasi tugas akhir dan tetap membuat verifikasi terasa
	// "~1 detik" sehingga brute force tidak murah.
	BiayaBCrypt = bcrypt.DefaultCost

	// PanjangTokenSesi adalah jumlah byte acak pada token sesi.
	// 32 byte = 256 bit, jauh di atas batas tebakan yang masuk akal.
	PanjangTokenSesi = 32

	// PanjangMinimumSandi adalah jumlah karakter minimum kata sandi.
	PanjangMinimumSandi = 6
)

// HashPassword mengembalikan hash bcrypt dari kata sandi.
//
// Setiap pemanggilan memakai salt baru, sehingga dua kata sandi yang sama
// menghasilkan hash yang berbeda. Ini penting: kalau hash sama untuk semua
// pengguna, bisa dilakukan rainbow table attack.
func HashPassword(sandi string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(sandi), BiayaBCrypt)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword memeriksa apakah sandi cocok dengan hash bcrypt.
//
// Fungsi selalu mengembalikan bool, tidak error: dari sisi pengguna tidak
// ada perbedaan antara "hash rusak" dan "sandi salah", dan pesan yang
// dikembalikan ke layar harus sama untuk keduanya.
func CheckPassword(hash, sandi string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(sandi)) == nil
}

// PasswordValid memeriksa apakah kata sandi memenuhi syarat minimum.
//
// Aturan ini hanya batas bawah: panjang, bukan kekuatan. Untuk aplikasi
// tugas akhir ini cukup, tanpa perlu menambah validator kekuatan
// password yang hanya memperbesar kode tanpa banyak manfaat.
func PasswordValid(sandi string) bool {
	return len(sandi) >= PanjangMinimumSandi
}

// TokenSesi menghasilkan token sesi acak sepanjang 32 byte dalam bentuk hex.
//
// Sumber acaknya crypto/rand, bukan math/rand, karena token ini adalah
// kredensial: bisa ditebak berarti orang lain bisa menyamar sebagai
// pengguna yang sedang login.
func TokenSesi() (string, error) {
	buf := make([]byte, PanjangTokenSesi)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HashToken mengubah token mentah menjadi SHA-256 dalam bentuk hex.
//
// Yang disimpan di tabel sesi adalah hasil fungsi ini, bukan token mentah.
// Kalau isi tabel sesi bocor, cookie yang sudah ada di browser tetap tidak
// bisa dipalsukan.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
