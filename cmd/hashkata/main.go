// Program hashkata mencetak hash bcrypt dari sebuah kata sandi.
//
// Dipakai untuk membuat hash pada seed akun demo di schema.sql dan
// migrate.sql, serta ketika admin perlu membuat kata sandi awal untuk
// akun kasir baru.
//
// Jalankan:  go run ./cmd/hashkata admin123
//
// Password asli tidak pernah disimpan di mana pun: yang dicetak hanya
// hash bcrypt-nya.
package main

import (
	"fmt"
	"os"

	"seduhpoin/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Pemakaian: go run ./cmd/hashkata <kata-sandi>")
		os.Exit(1)
	}
	sandi := os.Args[1]
	if !auth.PasswordValid(sandi) {
		fmt.Printf("Kata sandi minimal %d karakter.\n", auth.PanjangMinimumSandi)
		os.Exit(1)
	}

	hash, err := auth.HashPassword(sandi)
	if err != nil {
		fmt.Println("Gagal membuat hash:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
