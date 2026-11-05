# SeduhPoin — POS + Loyalty Program "Kedai Kopi Seduh"

Aplikasi web Point of Sale (POS) dengan program loyalitas untuk **Kedai Kopi Seduh**
(3 cabang). Dibuat sebagai tugas akhir menggunakan **Go murni** (tanpa framework web)
dan **MySQL** (XAMPP) tanpa ORM.

| Aspek | Teknologi |
|---|---|
| Bahasa | Go (paket standar + 3 dependensi) |
| Web server | `net/http` + `html/template` (bukan Gin/Echo/Fiber) |
| Database | MySQL (XAMPP/phpMyAdmin) via `database/sql` (bukan ORM) |
| Frontend | HTML + CSS murni (bukan React/Vue, tanpa build tool) |
| Autentikasi | Sesi server-side (tabel `sesi`) + kata sandi bcrypt |
| Dependensi | `github.com/go-sql-driver/mysql`, `github.com/joho/godotenv`, `golang.org/x/crypto` |

## Daftar Isi

| # | Bagian | Isi |
|---|---|---|
| 1 | [Setup dari Nol](#1-setup-dari-nol) | XAMPP, import `schema.sql`, isi `.env`, jalankan |
| 2 | [Daftar Menu](#2-daftar-menu) | Menu per peran (admin, kasir, member) |
| 3 | [Struktur Folder](#3-struktur-folder) | Peta kode & pola yang dipakai |
| 4 | [Aturan Bisnis](#4-aturan-bisnis-ringkasan) | Poin, tukar poin, 4 jenis promo, pembatalan |
| 5 | [Unit Test](#5-unit-test) | 15 test & cara menjalankannya |
| 6 | [Keputusan Desain](#6-keputusan-desain) | 18 keputusan + alasannya |
| 7 | [Catatan Teknis](#7-catatan-teknis) | Tipe data, timezone, atomisitas |
| 8 | [Kontribusi](#8-kontribusi-untuk-anggota-kelompok) | Anggota, alur branch & pull request |
| 9 | [Panduan Menjawab Pertanyaan Dosen](#9-panduan-menjawab-pertanyaan-dosen) | **Peta file + jawaban siap pakai** |
| 10 | [Batasan yang Ditegakkan](#10-batasan-yang-ditegakkan) | Larangan yang dipatuhi |

---

## 1. Setup dari Nol

### Langkah 1 — Jalankan MySQL XAMPP

1. Buka **XAMPP Control Panel** → klik **Start** pada modul **MySQL**.
2. Pastikan statusnya *Running* (port 3306).

Alternatif tanpa XAMPP Control Panel:

```bash
C:/xampp/mysql/bin/mysqld.exe --defaults-file=C:/xampp/mysql/bin/my.ini --console
```

### Langkah 2 — Buat database dan import skema

**Cara A — Import file `schema.sql` (paling cepat, disarankan)**

1. Buka <http://localhost/phpmyadmin>.
2. Klik tab **Import** di menu atas.
3. Klik **Choose File**, pilih `schema.sql` dari folder proyek, lalu klik **Go**.

Tidak perlu membuat database secara manual: `schema.sql` sudah memuat
`CREATE DATABASE`, `USE`, seluruh `CREATE TABLE` (InnoDB), index, foreign key,
serta data awal (seed).

**Cara B — Buat database dulu lewat tab SQL**

Kode pendek ini (tempel di tab **SQL**, lalu klik **Go**):

```sql
-- 1. Buat database
CREATE DATABASE IF NOT EXISTS `seduhpoin`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

-- 2. Pilih database
USE `seduhpoin`;

-- 3. (Opsional) User khusus aplikasi, lebih aman daripada memakai root
CREATE USER IF NOT EXISTS 'seduhpoin'@'localhost' IDENTIFIED BY 'seduhpoin123';
GRANT ALL PRIVILEGES ON `seduhpoin`.* TO 'seduhpoin'@'localhost';
FLUSH PRIVILEGES;
```

Bila memakai user khusus, sesuaikan `.env`:

```env
DB_DSN=seduhpoin:seduhpoin123@tcp(127.0.0.1:3306)/seduhpoin?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
```

Setelah database dibuat, **import `schema.sql`** (Cara A) untuk membuat tabel dan
mengisi data awal.

Verifikasi hasil import:

```sql
USE `seduhpoin`;
SHOW TABLES;                    -- 14 tabel (ada `pengguna` dan `sesi`)
SELECT COUNT(*) FROM cabang;    -- 3
SELECT COUNT(*) FROM produk;    -- 12
SELECT COUNT(*) FROM member;    -- 3
SELECT COUNT(*) FROM promo;     -- 5
SELECT COUNT(*) FROM voucher;   -- 1
SELECT username, role FROM pengguna;   -- admin (ADMIN), kasir (KASIR)
```

> **Penting:** `schema.sql` diawali `DROP TABLE IF EXISTS`, sehingga menjalankannya
> ulang akan **menghapus dan membuat ulang** seluruh tabel beserta datanya.
> Lakukan hanya saat setup pertama atau ketika ingin mengulang dari nol.

> **Database lama?** Jangan import ulang `schema.sql` (data lama hilang).
> Jalankan **`migrate.sql`** — berkas migrasi yang tidak menghapus apa pun,
> hanya menambah tabel/kolom baru (`pengguna`, `sesi`, `member.password_hash`,
> `transaksi.pengguna_id`, `transaksi.kasir_nama`) dan inserting akun demo.
> Migrasi ini idempotent, jadi aman dijalankan berkali-kali.

### Langkah 3 — Isi file `.env`

Salin `.env.example` menjadi `.env` lalu sesuaikan:

```env
DB_DSN=root:@tcp(127.0.0.1:3306)/seduhpoin?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
PORT=8080
```

> **Penting:** nilai `loc` harus ditulis `Asia%2FJakarta` (persen di-encode).
> Driver MySQL memisahkan DSN pada karakter `/` terakhir, sehingga `loc=Asia/Jakarta`
> akan membuat DSN gagal parsing (`invalid DSN: did you forget to escape a param value?`).

### Langkah 4 — Ambil dependensi & jalankan

```bash
go mod tidy
go run .
```

### Langkah 5 — Buka di browser

<http://localhost:8080>

Aplikasi memakai **login dengan tiga peran**. Halaman login terbuka di
<http://localhost:8080/login>.

| Peran | Username / No HP | Kata sandi awal | Keterangan |
|---|---|---|---|
| Admin / Pemilik | `admin` | `admin123` | Wajib mengganti kata sandi pada login pertama |
| Kasir | `kasir` | `kasir123` | Langsung bisa dipakai |
| Member | `081234567890` | `member123` | Tiga member seed: `081234567890`, `081298765432`, `085611223344` |

> Kata sandi disimpan sebagai **bcrypt hash** (bukan teks polos). Akun `admin`
> sengaja ditandai `harus_ganti_sandi`, sehingga setelah login pertama ia
> langsung diarahkan ke halaman ganti kata sandi dan belum bisa membuka menu
> lain sampai sandi diganti.

### Perintah yang sering dipakai

```bash
go build ./...        # cek kompilasi
go test ./...         # jalankan unit test
go test ./internal/calc -v   # unit test aturan bisnis (detail)
gofmt -l .            # cek format kode
```

---

## 2. Daftar Menu

Hak akses tiap menu ditegakkan **di server** pada middleware `a.akses(peran, ...)`
di titik registrasi rute. Menyembunyikan tautan di sidebar hanya untuk kenyamanan;
mengetik URL secara langsung tetap akan ditolak dengan halaman 403.

### Peran Admin / Pemilik

| Menu | URL | Keterangan |
|---|---|---|
| Dashboard | `/` | Ringkasan transaksi, promo aktif, member |
| Transaksi Baru | `/transaksi` | Form transaksi: pilih cabang, item, member, voucher, tukar poin |
| Riwayat Transaksi | `/riwayat` | Daftar transaksi + filter tanggal/cabang, detail, pembatalan |
| Produk | `/produk` | CRUD produk (soft delete) |
| Member | `/member` | CRUD member, kata sandi awal, reset kata sandi |
| Promo & Voucher | `/promo` | CRUD 4 jenis promo + pengelolaan kuota voucher |
| Cabang | `/cabang` | CRUD cabang (soft delete) |
| Poin Member | `/poin` | Saldo poin aktif, tingkat member, lot poin, mutasi poin |
| Laporan Biaya Promo | `/laporan/promo` | FR-18 |
| Laporan Poin Berlaku | `/laporan/poin` | FR-19 |
| Laporan Member per Tingkat | `/laporan/tingkat` | FR-20 |
| Kelola Pengguna | `/pengguna` | CRUD akun staf, reset kata sandi, aktif/nonaktifkan |
| Ganti Kata Sandi | `/akun/sandi` | Untuk semua peran |

### Peran Kasir

| Menu | URL | Keterangan |
|---|---|---|
| Transaksi Baru | `/transaksi` | Memproses transaksi: item, promo/voucher, tukar poin |
| Riwayat Transaksi | `/riwayat` | Melihat riwayat dan **membatalkan** transaksi |
| Produk | `/produk` | Hanya **membaca** katalog (untuk memilih item) |
| Member | `/member` | Hanya **membaca** data member |
| Promo & Voucher | `/promo` | Hanya **membaca** promo/voucher yang berlaku |
| Cabang | `/cabang` | Hanya **membaca** data cabang |
| Rincian Poin Member | `/poin/{id}` | Saldo & mutasi poin satu member |
| Ganti Kata Sandi | `/akun/sandi` | Untuk semua peran |

Menu **tidak** tersedia bagi kasir: Dashboard, Poin Member (daftar semua), seluruh
Laporan, dan Kelola Pengguna. Master data hanya bisa dibaca, tidak diubah.

### Peran Member / Pelanggan

Portal read-only di `/portal`:

| Menu | URL | Keterangan |
|---|---|---|
| Beranda Saya | `/portal` | Data keanggotaan, tingkat, saldo poin |
| Riwayat Transaksi Saya | `/portal/transaksi` | Riwayat transaksi miliknya saja |
| Promo & Voucher | `/portal/promo` | Promo/voucher yang sedang berlaku |
| Ganti Kata Sandi | `/akun/sandi` | Mengganti kata sandi sendiri |

Data awal (seed): nomor HP member `081234567890`, `081298765432`, `085611223344`;
kode voucher `SEDUH10K` (kuota 100).

---

## 3. Struktur Folder

```
SeduhPoin/
├── main.go                     koneksi DB, router, server
├── go.mod / go.sum
├── .env / .env.example
├── schema.sql                  DDL + index + seed data (installasi baru)
├── migrate.sql                 migrasi idempotent untuk database lama
├── README.md
├── cmd/
│   └── hashkata/               cetak hash bcrypt (untuk seed / reset sandi)
├── internal/
│   ├── auth/                   hash kata sandi + token sesi (bcrypt, SHA-256)
│   │   ├── password.go
│   │   └── password_test.go
│   ├── calc/                   ATURAN BISNIS (pure, unit-testable)
│   │   ├── calc.go             seluruh perhitungan & konstanta
│   │   └── calc_test.go        unit test wajib
│   ├── models/                 struktur domain
│   ├── format/                 pemformat angka, rupiah, tanggal (WIB)
│   └── store/                  akses database (SQL mentah, sql.Tx)
│       ├── db.go, cabang.go, produk.go, member.go,
│       ├── promo.go, poin.go, transaksi.go, laporan.go
│       └── auth.go             akun staf, akun member, tabel sesi
├── handlers/                   HTTP handler + template
│   ├── app.go, helpers.go, render.go
│   ├── auth.go                 login, logout, middleware peran
│   ├── akun.go                 ganti kata sandi (semua peran)
│   ├── pengguna.go             CRUD akun staf (khusus admin)
│   ├── portal.go               portal member (read-only)
│   ├── dashboard.go, cabang.go, produk.go, member.go, promo.go
│   ├── transaksi.go, riwayat.go, poin.go, laporan.go
├── templates/                  layout, header, sidebar, footer + halaman
│   ├── auth/                   halaman login
│   ├── akun/                   ganti kata sandi
│   ├── pengguna/               kelola akun staf
│   ├── portal/                 portal member
│   └── ...                     halaman operasional
└── static/style.css            seluruh gaya tampilan
```

Pola yang dipakai:

* `internal/calc` **tidak** menyentuh database sehingga seluruh aturan bisnis dapat
  diuji dengan `go test`. Fungsi `calc.Hitung()` dipakai oleh handler "Hitung"
  maupun saat menyimpan transaksi, sehingga perhitungan **tidak pernah diduplikasi**.
* `internal/store` menangani seluruh SQL; penulisan atomic memakai `sql.Tx`.
* `handlers` hanya mengatur HTTP, validasi input, dan pemanggilan template.
* `internal/auth` menangani hashing (bcrypt) dan pembuatan token sesi; tidak
  menyentuh database sehingga bisa diuji terpisah.

---

## 4. Aturan Bisnis (ringkasan)

Semua perhitungan berada di `internal/calc/calc.go`.

**Poin**

* 1 poin untuk setiap kelipatan **Rp10.000** dari total **setelah promo** (sisa di bawah
  Rp10.000 diabaikan).
* Dasar poin = total setelah promo; poin diperoleh = `floor(dasar x pengali)`.
* Pengali: **Silver x1.0**, **Gold x1.5**.
* Tingkat **tidak disimpan** di tabel member — dihitung on-the-fly dari total belanja
  3 bulan terakhir atas transaksi **SELESAI**. Ambang Gold: **Rp1.500.000**.
* Pengali yang dipakai sebuah transaksi adalah tingkat **pada saat transaksi dimulai**
  (sebelum transaksi itu ikut dihitung), dan disimpan pada
  `transaksi.tingkat_saat_transaksi` + `pengali_saat_transaksi`.
* Masa berlaku poin **6 bulan** (sampai akhir hari tanggal yang sama 6 bulan kemudian).
* Nilai tukar **1 poin = Rp1.000**.

**Tukar poin**

* Maksimal **50% dari total setelah promo**:
  `batas = min(saldo poin aktif, floor(50% x total setelah promo / 1.000))`.
* Penukaran memakai **FIFO**: lot dengan tanggal kedaluarsa terdekat dipakai lebih dulu;
  bila sama, yang diperoleh lebih dulu didahulukan.
* Hanya **member terdaftar** yang memperoleh dan menukar poin. Non-member tetap
  bisa memakai promo.

**Promo (4 jenis)**

| Jenis | Aturan |
|---|---|
| Diskon produk | Persentase diskon untuk produk tertentu |
| Potongan minimal belanja | Potongan nominal bila belanja mencapai minimum |
| Beli 2 gratis 1 | Setiap 3 unit dalam satu kategori → 1 unit termurah gratis (unit digabung per kategori) |
| Voucher | Kode unik, berkuota, maksimal 1 voucher per transaksi |

* Promo produk (diskon % dan B2G1) **boleh digabung**.
* Promo **potongan minimal belanja tidak boleh digabung dengan voucher**. Bila kode
  voucher diisi, potongan minimal belanja **tidak** diterapkan (BR-11).
* Promo memiliki masa berlaku, restriction cabang (kosong = semua cabang), dan
  rentang jam (kosong = sepanjang hari).
* Promo yang **sudah dipakai** hanya boleh **dinonaktifkan**, tidak dapat dihapus.

**Urutan perhitungan transaksi (WAJIB)**

1. `subtotal` = Σ (harga x jumlah)
2. Promo produk: **B2G1** dulu, lalu **diskon % produk**
3. **Salah satu**: voucher **ATAU** potongan minimal belanja (dihitung dari belanja
   setelah promo produk)
4. `totalSetelahPromo` (minimal Rp0) = dasar poin **dan** dasar batas tukar poin
5. Tukar poin (FIFO) → `totalBayar = totalSetelahPromo - poin x Rp1.000`
6. Setelah disimpan: `poinDiperoleh = floor(totalSetelahPromo / 10.000) x pengali`,
   lalu dibuat **lot poin baru** (kedaluarsa +6 bulan, status AKTIF)

**Pembatalan (BR-14)**

* Transaksi berstatus **BATAL** (pembatalan utuh, bukan parsial); data **tidak dihapus**.
* Poin yang diperoleh ditarik (lot → `DITARIK`, mutasi `TARIK`).
* Poin yang ditukar dikembalikan ke lot asalnya (mutasi `KEMBALI`).
* Kuota voucher dikembalikan (`terpakai - 1`).
* Biaya promo keluar dari laporan karena laporan hanya menghitung transaksi `SELESAI`.
* Pembatalan **ditolak** bila poin yang diperoleh dari transaksi tersebut sudah
  dipakai pada transaksi lain.

**Contoh angka (sudah menjadi unit test)**

```
Subtotal                Rp 145.000
- B2G1                  Rp  20.000
- Diskon produk          Rp   6.000
- Potongan min belanja   Rp  15.000
= Total setelah promo    Rp 104.000
Tukar 40 poin          - Rp  40.000
= Total bayar            Rp  64.000
Poin diperoleh = floor(104.000 / 10.000) x 1.5 (Gold) = 15 poin
```

---

## 5. Unit Test

```bash
go test ./internal/calc -v      # aturan bisnis (perhitungan transaksi & poin)
go test ./internal/models -v    # validitas promo (masa berlaku, jam, cabang, status)
```

Test yang tersedia:

| Test | Isi |
|---|---|
| `TestContohAngkaSubbab22` | Contoh angka wajib di atas (Rp145.000 → Rp104.000 → Rp64.000, 15 poin) |
| `TestContohAngkaTanpaMember` | Non-member memakai promo tanpa memperoleh/menukar poin |
| `TestVoucherTidakDigabungDenganPotonganMinBelanja` | BR-11: voucher dan potongan min belanja tidak digabung |
| `TestPoinMelebihiBatas` | Penukaran di atas batas 50% ditolak |
| `TestFIFODuaLot` | FIFO 2 lot + lot kedaluwarsa dilewati |
| `TestFIFOSamaKedaluarsa` | Tie-break FIFO: yang diperoleh lebih dulu |
| `TestTingkatGoldBoundary` | Rp1.499.999 = Silver, Rp1.500.000 = Gold |
| `TestPoinDiperoleh` | Dasar poin dan pengali tingkat |
| `TestBatasTukarPoin` | Batas tukar 50% |
| `TestTanggalKedaluarsa` | Masa berlaku 6 bulan (termasuk 31 Agu → 28 Feb) |
| `TestB2G1GabungKategori` | Unit digabung dalam satu kategori |
| `TestBerlakuPadaMasaBerlaku` | Promo hanya berlaku antara `tgl_mulai` dan `tgl_selesai` |
| `TestBerlakuPadaJam` | Promo berkode jam (kosong = sepanjang hari) |
| `TestBerlakuPadaCabang` | Promo berkode cabang (kosong = semua cabang) |
| `TestBerlakuPadaStatusAktif` | Promo nonaktif tidak dapat dipakai |

---

## 6. Keputusan Desain

1. **Autentikasi berbasis sesi di server** — ada tabel `pengguna` (akun staf
   ADMIN/KASIR) dan kolom `password_hash` pada `member` (akun pelanggan).
   Kata sandi di-hash dengan **bcrypt**, sesi disimpan di tabel `sesi`, dan
   hak akses tiap rute diperiksa di middleware `wajibPeran`.
2. **Tingkat member tidak disimpan** — dihitung on-the-fly dari transaksi 3 bulan
   terakhir, sehingga selalu akurat tanpa perlu job pemeliharaan data.
3. **`cabang_ids` disimpan sebagai CSV** (`VARCHAR`) pada tabel `promo`; kosong berarti
   berlaku untuk semua cabang. Cukup untuk 3 cabang dan menghindari tabel pivot.
4. **Jam promo disimpan sebagai string `"HH:MM:SS"`** pada kolom `TIME`, agar tidak
   bermasalah dengan parsing driver MySQL.
5. **Soft delete** untuk master data (kolom `deleted_at`); nama produk, harga, dan nama
   member **disalin** ke tabel transaksi agar transaksi lama tetap menampilkan data lama.
6. **Hapus master data tidak menghapus data historis** — seluruh transaksi lama tetap
   utuh dan terbaca di riwayat maupun laporan.
7. **Pemotongan promo produk** — satu produk yang terkena beberapa promo diskon % memakai
   **persentase terbesar**, dan potongan diatribusikan ke promo tersebut agar laporan
   biaya promo (FR-18) akurat.
8. **Biaya tukar poin pada laporan biaya promo** ditampilkan pada **kolom terpisah**
   dari total potongan promo, dan tidak dihitung dua kali bila satu transaksi memakai
   beberapa promo.
9. **`voucher_kode` disimpan pada tabel `transaksi`** agar kuota voucher dapat dikembalikan
   dengan tepat saat pembatalan.
10. **Kuota voucher dijaga dengan satu pernyataan**:
    `UPDATE voucher SET terpakai = terpakai + 1 WHERE kode = ? AND terpakai < kuota`,
    lalu jumlah baris yang terpengaruh diperiksa — sehingga kuota tidak mungkin terpakai
    melebihi kuota.
11. **Penguncian baris poin** memakai `SELECT ... FOR UPDATE` pada tabel `poin_lot`
    sehingga satu lot tidak dipakai dua kali oleh transaksi bersamaan.
12. **Tanpa `log.Fatal`** — kegagalan database tidak mematikan server; pengguna melihat
    halaman pesan yang jelas (503), server tetap berjalan.
13. **Zona waktu Asia/Jakarta** dijamin dengan `import _ "time/tzdata"` (Windows sering
    tidak menyediakan basis data zona waktu sistem) dan `loc=Asia%2FJakarta` pada DSN.
14. **Template diparse per-request** (tanpa cache) supaya perubahan tampilan langsung
    terlihat tanpa perlu restart server.
15. **JavaScript diminimalkan** — hanya satu fungsi untuk menambah baris item dan
    konfirmasi pembatalan. Perhitungan tidak pernah dilakukan di sisi browser: tombol
    "Hitung" mengirim POST ke server lalu server menampilkan kembali rinciannya.
16. **Sidebar dapat dibuka/tutup di layar kecil** memakai checkbox CSS murni
    (tanpa JavaScript), sehingga menu tetap bisa diakses di ponsel.
17. **Berkas statis memakai `Cache-Control: no-cache`** agar perubahan tampilan
    langsung terlihat tanpa perlu memaksa refresh keras di browser.
18. **Ikon memakai SVG inline** pada sidebar dan header, bukan font ikon pihak
    ketiga, sehingga tidak ada permintaan jaringan tambahan dan tetap sesuai
    larangan penggunaan framework frontend.
19. **Pembagian peran sesuai tanggung jawab** — admin mengelola master data,
    cabang, promo, member, laporan, dan akun staf; kasir memproses transaksi,
    menerapkan promo, memakai poin, dan membatalkan transaksi; member hanya
    melihat data keanggotaannya sendiri (read-only).
20. **Master data bisa dibaca kasir, tetapi tidak diubah** — kasir tetap
    membutuhkan katalog produk, data member, dan daftar promo saat melayani,
    tetapi setiap rute tulis (tambah/ubah/hapus) memakai `peranAdmin`.
21. **Akun `admin` demo dipaksa mengganti kata sandi** — kolom
    `harus_ganti_sandi` membuat middleware menahan semua halaman selain
    `/akun/sandi` sampai kata sandi awal diganti.

---

## 7. Catatan Teknis

* Semua nilai uang dan poin bertipe `int64` (tanpa `float` untuk nominal).
* **Tampilan**: satu berkas `static/style.css` (tanpa framework CSS) berisi
  design system sederhana: palet espresso + krem, sidebar bertingkat, header
  lengket, kartu statistik, tabel dengan header lengket, dan aturan responsif
  (sidebar menjadi drawer di layar < 900px).
* Semua waktu disimpan dan ditampilkan dalam zona **WIB (Asia/Jakarta)**.
* Penulisan transaksi dan pembatalan masing-masing berjalan dalam **satu `sql.Tx`**:
  transaksi, item, rincian promo, penggunaan voucher, mutasi tukar poin, dan pembuatan
  lot poin baru disimpan secara atomik.
* Waktu server (`created_at`, `tanggal_transaksi`, `batal_at`) menggunakan waktu
  aplikasi di zona WIB.

---

## 8. Kontribusi (untuk anggota kelompok)

### Anggota Kelompok 5

| Nama | NIM |
|---|---|
| Nabilah Aulia Zahra | 2404130099 |
| Rahma Pratiwi | 2404130055 |
| Luhur Pangaribowo H | 2404130093 |
| Adinda Anisa Rizkyani Santoso | 2404130133 |
| Hilal Iqbaal Tawakal | 2404130166 |

Program studi Teknik Informatika, FMIPA, Universitas Negeri Semarang.

### Alur kerja bersama

1. **Clone** repositori:

   ```bash
   git clone https://github.com/Luuhhuurr/SeduhPoin.git
   cd SeduhPoin
   ```

2. **Siapkan lingkungan** (lihat [Bagian 1](#1-setup-dari-nol)): jalankan MySQL XAMPP,
   import `schema.sql`, lalu salin `.env.example` menjadi `.env` dan sesuaikan
   `DB_DSN` dengan setting MySQL lokal Anda.

   > **Penting:** file `.env` **tidak** ikut masuk ke repositori (sudah masuk
   > `.gitignore`) karena berisi data akses database. Setiap anggota membuat
   > `.env` sendiri.

3. **Jalankan aplikasi:**

   ```bash
   go mod tidy
   go run .
   ```

4. **Kirim perubahan** lewat *pull request* agar bisa direview classmates:

   ```bash
   git pull                      # ambil perubahan terbaru
   git checkout -b fitur/nama    # buat branch baru
   # ... ubah kode ...
   go build ./... && go test ./...   # pastikan lulus sebelum push
   git commit -am "deskripsi perubahan"
   git push origin fitur/nama
   ```

   Lalu buka **Pull request** di halaman GitHub repositori.

### Aturan main bersama

* **Selalu** jalankan `go build ./...` dan `go test ./...` sebelum push.
* **Jangan** mengubah `internal/calc` secara semantik tanpa menambah/memperbarui
  unit test di `internal/calc/calc_test.go` — aturan bisnis di sana adalah
  acuan semua anggota.
* Format kode dijaga dengan `gofmt -l .` (hasilnya harus kosong).
* Jangan pernah commit `.env`, file `*.exe`, atau kredensial apa pun.

---

## 9. Panduan Menjawab Pertanyaan Dosen

Bagian ini adalah peta: **pertanyaan yang sering muncul** → **jawaban singkat**
→ **file & fungsi yang ditunjuk** saat dosen bertanya. Semua kode sudah diberi komentar
penjelas, jadi namun jawabannya selalu bisa ditunjuk ke baris kodenya.

### 9.0 Peta file: "kalau ditanya, buka file ini"

| File | Isi | Pertanyaan yang bisa dijawab dari sini |
|---|---|---|
| `main.go` | Koneksi DB, router, server | Konfigurasi `.env`, timezone WIB, kenapa tidak `log.Fatal` |
| `handlers/app.go` | Registrasi seluruh rute | Alur request, inti RBAC per rute |
| `handlers/transaksi.go` | Form transaksi, Hitung, Simpan | Alur transaksi, kenapa Hitung = POST |
| `handlers/riwayat.go` | Riwayat + pembatalan | Filter tanggal/cabang, BR-14 |
| `handlers/promo.go` | CRUD 4 jenis promo | Validasi promo, kenapa promo terpakai tak bisa dihapus |
| `handlers/poin.go` | Saldo & mutasi poin | Kenapa tingkat dihitung on-the-fly, kenapa saldo tak disimpan |
| `handlers/laporan.go` | Tiga laporan | FR-18/19/20 |
| `handlers/member.go`, `produk.go`, `cabang.go` | Master data | Soft delete, unik no_hp, harga > 0 |
| `handlers/auth.go` | Login, logout, middleware peran | Kenapa sesi di server, bagaimana RBAC ditegakkan |
| `handlers/pengguna.go` | CRUD akun staf (admin) | Kenapa kasir tak boleh kelola akun staf |
| `handlers/portal.go` | Portal member | Kenapa read-only, bagaimana transaksi milik sendiri dibatasi |
| `handlers/akun.go` | Ganti kata sandi | Kenapa sandi lama wajib, kenapa sesi ikut dicabut |
| `internal/auth/password.go` | bcrypt + token sesi | Bagaimana password disimpan, kenapa token di-hash |
| `internal/store/auth.go` | Akun staf, akun member, tabel sesi | Sesi server-side, invalidasi saat logout |
| `handlers/render.go`, `helpers.go` | Template & bantuan | Parse per request, validasi, pesan error |
| `internal/calc/calc.go` | **Seluruh aturan bisnis** | Semua pertanyaan hitungan: B2G1, diskon, voucher, poin, FIFO, tingkat |
| `internal/calc/calc_test.go` | Unit test aturan bisnis | "Ini dibuktikan test apa?" |
| `internal/store/transaksi.go` | Simpan & batalkan transaksi | `sql.Tx`, `FOR UPDATE`, kuota voucher |
| `internal/store/poin.go` | Lot & mutasi poin | FIFO, saldo aktif on-the-fly |
| `internal/store/promo.go` | Promo & voucher | Syarat berlaku, soft delete |
| `internal/store/laporan.go` | Query 3 laporan | Hitungan biaya promo & poin |
| `internal/store/member.go` | Member & belanja 3 bulan | Perhitungan tingkat Silver/Gold |
| `internal/models/models.go` | Struct domain | Kenapa tidak ada kolom tingkat/saldo |
| `internal/models/models_test.go` | Test validitas promo | Masa berlaku, jam, cabang, status |
| `internal/format/format.go` | Rupiah & tanggal WIB | Format tampilan |
| `schema.sql` | DDL + index + seed | Kenapa InnoDB, kenapa `CHECK`, index apa saja |
| `static/style.css` | Seluruh tampilan | Tanpa framework CSS, responsif |
| `templates/` | Halaman HTML | Layout bersama, `{{define "content"}}` |

### 9.1 Pertanyaan arsitektur

| Pertanyaan | Jawaban singkat | File & fungsi |
|---|---|---|
| Kenapa tidak pakai framework web? | Spesifikasi tugas melarang; `net/http` + `html/template` saja sudah cukup. Router memakai pola `ServeMux` Go 1.22+ (method + wildcard). | `main.go`, `handlers/app.go` → `App.Register` |
| Kenapa tidak pakai ORM? | Dilarang spesifikasi. Semua akses data lewat `database/sql` dengan SQL mentah; tiap query ditulis tangan di layer `store`. | `internal/store/db.go` → `Store`, `Store.Query`, `Store.ScanRows` |
| Kenapa login memakai sesi di database, bukan cookie biasa? | **Sudah ada login** dengan tiga peran (admin, kasir, member). Autentikasi memakai sesi server-side di tabel `sesi` dan kata sandi bcrypt. | `handlers/auth.go` → `prosesLogin`, `buatSesi`; `internal/store/auth.go` |
| Bagaimana hak akses tiap peran ditegakkan? | Setiap rute didaftarkan dengan `a.akses(peran, handler)`. Middleware `wajibPeran` memeriksa peran dari sesi; bila tidak cocok, server membalas 403 — bukan sekadar menyembunyikan menu di sidebar. | `handlers/app.go` → `App.Register`; `handlers/auth.go` → `akses`, `wajibPeran`, `Identitas.Boleh` |
| Bagaimana kata sandi disimpan? | Dengan **bcrypt** (`golang.org/x/crypto/bcrypt`, cost 10). Database hanya menyimpan hash; kata sandi polos tidak pernah ditulis ke database. | `internal/auth/password.go` → `HashPassword`, `CheckPassword` |
| Kenapa sesi disimpan di database, bukan di cookie saja? | Supaya logout benar-benar mencabut akses (baris sesi dihapus) dan akun yang dinonaktifkan langsung kehilangan sesi. Cookie hanya menyimpan token acak; yang tersimpan di database adalah **SHA-256(token)**. | `handlers/auth.go` → `buatSesi`, `hapusSesi`; `internal/store/auth.go` → `BuatSesi`, `AmbilSesi` |
| Kenapa kasir tak boleh mengelola akun staf? | Managementi akun staf memengaruhi akses sistem secara global (bisa membuat akun admin baru), pemisahan ini mencegah eskalasi hak akses dari kasir ke admin. | `handlers/pengguna.go` → `registerPengguna` (semua rute `peranAdmin`) |
| Kenapa member hanya punya akses read-only? | Portal member untuk melihat data keanggotaan, poin, tingkat, dan riwayat transaksinya sendiri — seluruh data itu sudah tersedia dari sisi server. Menulis dari sisi pelanggan berarti memindahkan wewenang bisnis ke luar kasir. | `handlers/portal.go` |
| Kenapa uangnya `int64`, bukan `float`? | `float64` menimbulkan galat pembulatan pada nominal rupiah. Semua uang dan poin `int64`; `float64` hanya untuk pengali tingkat (1.5 / 1.0). | `internal/calc/calc.go` → konstanta di bagian atas |
| Bagaimana pemisahan layer? | `handlers` = HTTP + validasi + template; `internal/store` = seluruh SQL; `internal/calc` = aturan bisnis murni; `internal/models` = struct; `internal/format` = format angka/tanggal. `calc` tidak boleh menyentuh database. | Seluruh folder `internal/` |
| Kenapa `internal/calc` dipisah? | Supaya aturan bisnis bisa diuji tanpa database. Semua perhitungan di satu tempat, dipakai ulang oleh tombol "Hitung" maupun saat menyimpan. | `internal/calc/calc.go` → `Hitung` |

### 9.2 Pertanyaan aturan bisnis

| Pertanyaan | Jawaban singkat | File & fungsi |
|---|---|---|
| Bagaimana urutan perhitungan transaksi? | Wajib: subtotal → B2G1 → diskon % produk → (voucher **atau** potongan min belanja) → total setelah promo → tukar poin → poin diperoleh. | `internal/calc/calc.go` → `Hitung` (nomor langkah ada di komentar func) |
| Kenapa "Hitung" pakai POST ke server, bukan JavaScript? | Agar perhitungan tidak diduplikasi. Fungsi `calc.Hitung` yang sama dipakai saat menampilkan rincian dan saat menyimpan, jadi angka di struk pasti sama. | `handlers/transaksi.go` → `hitungTransaksi`, `renderFormTransaksi` |
| Bagaimana batas tukar poin? | Maksimal 50% dari total **setelah promo**: `min(saldo aktif, floor(50% × total / Rp1.000))`. | `internal/calc/calc.go` → `BatasTukarPoin` |
| Mengapa tingkat member tidak disimpan di tabel member? | Dihitung on-the-fly dari total belanja 3 bulan terakhir (hanya transaksi `SELESAI`). Jadi selalu akurat tanpa job pemeliharaan data. Ambang Gold Rp1.500.000. | `internal/store/member.go` → `Belanja3Bulan`, `Belanja3BulanSemuaMember`; `internal/calc/calc.go` → `TingkatMember` |
| Kenapa pengali disimpan di tabel transaksi? | Agar perolehan poin tidak berubah retroactive ketika member naik tingkat di kemudian hari. Menyimpan snapshot tingkat saat transaksi dimulai. | `internal/store/transaksi.go` → `DataSimpanTransaksi.Tingkat`; kolom `tingkat_saat_transaksi` |
| Bagaimana FIFO poin bekerja? | Poin disimpan per *lot* (batch) dengan tanggal kedaluwarsa sendiri. Tukar poin memakai lot yang **paling dekat kedaluwarsa** dulu; jika sama, yang diperoleh lebih dulu didahulukan. | `internal/calc/calc.go` → `UrutFIFO`, `AlokasiFIFO`; `internal/store/poin.go` → `AmbilLotAktifTx` |
| Kenapa saldo poin tidak disimpan? | Karena tiap poin punya masa berlaku berbeda (6 bulan), saldo harus dijumlahkan dari lot yang masih `AKTIF` dan belum kedaluwarsa. | `internal/store/poin.go` → `SaldoPoinAktif` |
| Kenapa tidak boleh digabung voucher + potongan min belanja? | Aturan BR-11. Bila kode voucher diisi, potongan minimal belanja dilewati; kode voucher hanya satu per transaksi. | `internal/calc/calc.go` → `Hitung`, bagian voucher ATAU min belanja |
| Bagaimana B2G1 menghitung unit gratis? | Unit digabung per kategori; tiap 3 unit → 1 unit **termurah** gratis. Diskon % tidak diterapkan pada unit gratis. | `internal/calc/calc.go` → `HitungB2G1`, `HitungDiskonProduk` |
| Kapan sebuah promo berlaku? | Ada empat syarat: masa berlaku tanggal, jam (kosong = sepanjang hari), cabang (kosong = semua cabang), dan status aktif. | `internal/models/models.go` → `Promo.BerlakuPada`; `internal/store/promo.go` → `PromoAktif` |
| Kenapa promo yang sudah dipakai tidak boleh dihapus? | Agar laporan biaya promo (FR-18) tetap punya rujukan promo yang valid. Promo yang terpakai hanya boleh dinonaktifkan. | `internal/store/promo.go` → `PromoSudahDipakai`; `handlers/promo.go` → `hapusPromo` |
| Bagaimana pembatalan transaksi? | Status → `BATAL`, poin perolehan ditarik, poin yang ditukar dikembalikan ke lot asal, kuota voucher dikembalikan. Data **tidak** dihapus. Ditolak bila poin perolehan sudah dipakai di transaksi lain. | `internal/store/transaksi.go` → `BatalkanTransaksi` |

### 9.3 Pertanyaan non-functional / keamanan

| Pertanyaan | Jawaban singkat | File & fungsi |
|---|---|---|
| Bagaimana menjamin simpan transaksi itu atomic? | Semua penulisan (header, item, rincian promo, voucher, tukar poin, lot baru) dijalankan dalam **satu `sql.Tx`**, dengan `defer tx.Rollback()` dan `Commit()` di akhir. | `internal/store/transaksi.go` → `SimpanTransaksi` |
| Bagaimana mencegah dua kasir memakai lot poin yang sama? | Lot dikunci dengan `SELECT ... FOR UPDATE` di dalam transaksi yang sama, sehingga lot tidak bisa diubah transaksi lain sampai `Commit`. | `internal/store/poin.go` → `AmbilLotAktifTx` |
| Bagaimana menjamin kuota voucher tidak terlampaui? | Satu statement: `UPDATE voucher SET terpakai = terpakai + 1 WHERE kode = ? AND terpakai < kuota`, lalu `RowsAffected()` diperiksa. Ada pula `CHECK (terpakai <= kuota)` di database. | `internal/store/transaksi.go` → `SimpanTransaksi`; `schema.sql` → tabel `voucher` |
| Kenapa tidak pakai `log.Fatal`? | `log.Fatal` mematikan seluruh server saat satu request gagal. Di sini kegagalan database hanya menghasilkan halaman pesan, server tetap melayani request lain. | `handlers/app.go`, `handlers/helpers.go` → `pesanError` |
| Bagaimana validasi input & pesan error? | Validasi di server (bukan hanya di browser), dengan pesan Bahasa Indonesia yang jelas; form di-render ulang dengan isian yang tadi. | `handlers/helpers.go` → `parseInt64`, `parseTanggal`; `handlers/promo.go` → `validasiPromo` |
| Kenapa master data di-soft delete? | Transaksi lama harus tetap menampilkan nama produk/member lama. Karena itu nama & harga juga **disalin** ke tabel `transaksi_item` saat transaksi disimpan. | `internal/store/{produk,member,cabang}.go`; `schema.sql` → tabel `transaksi_item` |
| Bagaimana zona waktu dijamin WIB? | `import _ "time/tzdata"` di `main.go` (Windows sering tidak punya tzdata sistem) + `loc=Asia%2FJakarta` pada DSN. | `main.go`; `internal/format/format.go` → `WIB` |
| Kenapa DSN harus `Asia%2FJakarta`? | Driver MySQL memisahkan DSN pada karakter `/` terakhir, jadi `/` harus di-encode menjadi `%2F`. | README Bagian 1, Langkah 3 |
| Bagaimana template dinamis? | Template di-parse **per request** tanpa cache, jadi mengubah `.html` langsung terlihat tanpa restart. | `handlers/render.go` |
| Apa yang dijamin unit test? | Contoh angka wajib (Rp145.000 → Rp104.000 → Rp64.000, 15 poin), FIFO 2 lot + tie-break, batas Gold (Rp1.499.999 Silver / Rp1.500.000 Gold), syarat gabung promo, validitas promo, dan hashing password (bcrypt + SHA-256 token). | `internal/calc/calc_test.go`, `internal/models/models_test.go`, `internal/auth/password_test.go` |
| Bagaimana cookie sesi dibuat aman? | `HttpOnly` (tidak bisa dibaca JavaScript), `SameSite=Lax` (proteksi dasar CSRF), dan `Secure` hanya aktif bila koneksi HTTPS — demo lokal jalan di HTTP sehingga cookie `Secure` akan ditolak browser. | `handlers/auth.go` → `buatSesi` |
| Bagaimana member hanya melihat transaksinya sendiri? | Query detail transaksi memfilter `member_id` dari sesi (bukan dari URL), dan transaksi milik orang lain dibalas 404 — bukan 403 — agar keberadaannya tidak bocor. | `handlers/portal.go` → `portalDetailTransaksi`; `internal/store/transaksi.go` → `AmbilTransaksiMilikMember` |
| Bagaimana logout dan ganti sandi mencabut akses? | Logout menghapus baris sesi di database, sehingga token langsung tidak berlaku. Ganti sandi juga menghapus **semua** sesi akun tersebut lalu membuat sesi baru untuk pengguna yang sedang mengubah sandi. | `handlers/auth.go` → `hapusSesi`; `handlers/akun.go` → `prosesGantiSandi` |

### 9.4 Alur demo yang aman (tidak merusak data demo)

Semua langkah diawali login. Gunakan akun `admin`/`admin123` (wajib ganti sandi
pada login pertama), `kasir`/`kasir123`, atau nomor HP member + `member123`.

Kalau diminta demo cepat, urutan ini paling aman dan tidak merusak data:

1. **Login** — tunjukkan halaman login dan peran yang tersedia.
2. **Dashboard (admin)** — tunjukkan ringkasan (menu paling atas).
3. **Transaksi → Hitung** (tanpa menyimpan) — masukkan 3 item Kopi Susu Gula Aren
   (Rp28.000), pakai voucher `SEDUH10K`, tekan **Hitung**. Tunjukkan rinciannya:
   subtotal, potongan, total setelah promo, batas tukar poin, poin yang diperoleh.
   **Jangan klik Simpan** supaya data demo tetap bersih.
4. **Promo** — tunjukkan 4 jenis promo dan tanggal/jam/cabang; jelaskan kenapa
   diskon produk boleh digabung tapi min-belanja tidak boleh digabung voucher.
5. **Poin Member** — buka `081234567890`, jelaskan saldo = jumlah lot aktif,
   tingkat dihitung on-the-fly, dan mutasi `PEROLEHAN/PENUKARAN/KEMBALI/TARIK`.
6. **Laporan** — tunjukkan tiga laporan (biaya promo, poin berlaku, member per tingkat).
7. **Portal member** — login sebagai `081234567890`/`member123`, tunjukkan saldo
   poin, tingkat keanggotaan, dan riwayat transaksi sendiri.

> Untuk menunjukkan pembatasan peran, buka `/pengguna` (Kelola Pengguna) memakai
> akun kasir — server membalas **403 Akses ditolak**, walaupun tautannya memang
> tidak ditampilkan di sidebar.

> Kalau diminta demo **penyimpanan** transaksi, lakukan di cabang `C03` (bandung)
> dengan kode voucher `SEDUH10K`, lalu batalkan kembali lewat menu Riwayat agar
> tampilan kembali bersih. Pembatalan juga sekaligus menunjukkan fitur BR-14.

---

## 10. Batasan yang Ditegakkan

* Tidak memakai framework web (Gin/Echo/Fiber/Beego).
* Tidak memakai ORM (GORM atau lainnya); seluruh akses database memakai `database/sql`
  dengan SQL mentah.
* Tidak memakai React/Vue/Angular atau build tool pada frontend.
* Autentikasi memakai pustaka standar `golang.org/x/crypto/bcrypt` (bukan
  implementasi buatan sendiri) dan sesi disimpan di database — bukan cookie
  berisi data identitas.
* Seluruh perhitungan aturan bisnis berada di `internal/calc` dan dapat diuji dengan
  `go test`.
