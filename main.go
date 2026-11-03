// Program utama aplikasi web SeduhPoin.
//
// SeduhPoin adalah aplikasi POS + loyalty program untuk Kedai Kopi Seduh.
// Menjalankan:  go run .   lalu buka http://localhost:8080
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// tzdata ensures zona waktu Asia/Jakarta selalu tersedia,
	// termasuk di Windows yang tidak menyediakan basis data zona waktu sistem.
	_ "time/tzdata"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"seduhpoin/handlers"
	"seduhpoin/internal/store"
)

func main() {
	// 1. Ambil konfigurasi dari file .env
	if err := godotenv.Load(); err != nil {
		fmt.Println("Perhatian: file .env tidak terbaca, memakai nilai bawaan.")
	}
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/seduhpoin?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 2. Koneksi database. sql.Open bersifat lazy, lalu kita coba ping.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("Gagal menyiapkan koneksi database:", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Println("Peringatan: database belum dapat dijangkau:", err)
		fmt.Println("Pastikan MySQL XAMPP berjalan dan DB_DSN pada .env sudah benar.")
		fmt.Println("Server tetap dijalankan; halaman akan menampilkan pesan koneksi gagal.")
	}

	// 3. Siapkan aplikasi dan router
	st := store.New(db)
	app := handlers.New(st)

	mux := http.NewServeMux()
	app.Register(mux)

	// Berkas statis (CSS). Cache-Control no-cache membuat browser selalu
	// memvalidasi ulang berkas, sehingga perubahan tampilan langsung terlihat.
	berkasStatis := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		berkasStatis.ServeHTTP(w, r)
	}))

	// 4. Jalankan server
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("SeduhPoin berjalan di http://localhost:%s\n", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// Server berhenti karena error tak terduga; catat tanpa mematikan proses lain.
		log.Printf("Server berhenti: %v", err)
	}
}
