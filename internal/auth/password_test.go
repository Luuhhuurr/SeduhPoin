package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordTidakMenyimpanSandiPolos(t *testing.T) {
	hash, err := HashPassword("admin123")
	if err != nil {
		t.Fatalf("HashPassword gagal: %v", err)
	}
	if hash == "admin123" {
		t.Fatal("hash tidak boleh sama dengan kata sandi polos")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash bcrypt harus diawali $2,Got: %q", hash)
	}
}

func TestHashPasswordMemakaiSaltBerbeda(t *testing.T) {
	sandi := "member123"
	hash1, _ := HashPassword(sandi)
	hash2, _ := HashPassword(sandi)
	if hash1 == hash2 {
		t.Fatal("dua hash untuk sandi sama harus berbeda (salt acak)")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("kasir123")

	if !CheckPassword(hash, "kasir123") {
		t.Fatal("sandi yang benar harus diterima")
	}
	if CheckPassword(hash, "kasir124") {
		t.Fatal("sandi yang salah harus ditolak")
	}
	if CheckPassword("", "kasir123") {
		t.Fatal("hash kosong harus ditolak")
	}
	if CheckPassword("bukan-hash-bcrypt", "kasir123") {
		t.Fatal("hash rusak harus ditolak, bukan dianggap benar")
	}
}

func TestPasswordValid(t *testing.T) {
	if PasswordValid("abc") {
		t.Fatal("sandi pendek harus ditolak")
	}
	if !PasswordValid("abc123") {
		t.Fatal("sandi 6 karakter harus diterima")
	}
}

func TestTokenSesiBerbedaDanPanjang64(t *testing.T) {
	t1, err := TokenSesi()
	if err != nil {
		t.Fatalf("TokenSesi gagal: %v", err)
	}
	t2, _ := TokenSesi()

	if t1 == t2 {
		t.Fatal("dua token sesi harus berbeda")
	}
	if len(t1) != PanjangTokenSesi*2 {
		t.Fatalf("panjang token hex harus %d, Got: %d", PanjangTokenSesi*2, len(t1))
	}
}

func TestHashToken(t *testing.T) {
	token := "token-mentah-123"
	h := HashToken(token)

	if len(h) != 64 {
		t.Fatalf("hash SHA-256 hex harus 64 karakter, Got: %d", len(h))
	}
	if h == token {
		t.Fatal("hash token tidak boleh sama dengan token mentah")
	}
	if HashToken(token) != h {
		t.Fatal("HashToken harus deterministik")
	}
	if HashToken("token-lain") == h {
		t.Fatal("token berbeda harus menghasilkan hash berbeda")
	}
}
