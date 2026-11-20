# Delcom Blog — Backend REST API (Golang Gin)

Studi Kasus PABWE 2026 Praktikum 9. REST API untuk platform blog multi-tier (Publik, Penulis, Admin) dengan **AI Blog Generator yang terintegrasi mesin pencari**, dibangun dengan Go 1.27, Gin, GORM, PostgreSQL 17, arsitektur **Feature-First & Clean Layering**, dan **TDD (100% statement coverage)**.

Frontend pasangan: `ifs24044-pabwe2026-sk-p9-fe-reactjs`.

## Fitur

- **Publik** — daftar kategori, galeri artikel terbit dengan pencarian (`?q=`) dan filter kategori (`?category=`), detail artikel via slug.
- **Penulis** — CRUD artikel milik sendiri, slug unik otomatis (`judul`, `judul-2`, …), estimasi waktu baca (kata ÷ 200 wpm, minimal 1 menit), ringkasan otomatis bila kosong, toggle Draf ↔ Terbit (mengatur `published_at`), statistik Total/Terbit/Draf.
- **AI Blog Generator** — topik → penelusuran mesin pencari → prompt + konteks ke OpenAI Chat Completions → draf lengkap (judul, kategori, excerpt, konten Markdown bersitasi, daftar sumber).
- **Admin** — direktori pengguna beserta jumlah artikel, arsip seluruh artikel, artikel per pengguna.
- **Swagger UI** di `/swagger/index.html`, spesifikasi di `/api.json` (host & scheme menyesuaikan URL pemanggil).
- Format respons seragam `{ "status": "success|fail|error", "message": "...", "data": ... }` dan handler 404 `Endpoint not found`.

## Konfigurasi (`.env`)

Salin `.env.example` menjadi `.env`, lalu sesuaikan.

| Variabel | Keterangan |
|---|---|
| `APP_ENV` | `development` / `production` (mode rilis Gin) |
| `APP_PORT` | Port HTTP (default `8081`; fallback ke `PORT` bila ada) |
| `APP_KEY` | Kunci penandatangan JWT (HS256) — **wajib diganti di produksi** |
| `ADMIN_USERNAME`, `ADMIN_PASSWORD` | Akun admin yang dibuat seeder |
| `DB_CONNECTION` | `pgsql` (default) atau `sqlite` untuk mencoba tanpa PostgreSQL |
| `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`, `DB_SSLMODE` | Koneksi PostgreSQL |
| `SEARCH_ENGINE_ENDPOINT` | Mesin pencari utama. Bila gagal/kosong, otomatis memakai Wikipedia (`id.wikipedia.org`) |
| `OPENAI_ENDPOINT`, `OPENAI_KEY`, `OPENAI_MODEL` | Endpoint OpenAI-compatible untuk AI generator |

Catatan:
- `OPENAI_KEY` hanya diteruskan ke mesin pencari bila host-nya sama dengan `OPENAI_ENDPOINT`, sehingga kunci tidak bocor ke layanan pihak ketiga.
- Bila layanan AI tidak tersedia, `POST /api/posts/ai-generate` tetap mengembalikan **kerangka draf** berbasis hasil penelusuran dengan `"generator": "fallback"` dan pesan yang menjelaskannya — UI menampilkan peringatan agar penulis tahu draf bukan buatan AI.

## Menjalankan

```bash
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"
cp .env.example .env
go run .                    # atau: go run cmd/server/main.go
```

- API: `http://localhost:8081/api`
- Swagger UI: `http://localhost:8081/swagger/index.html`

Tanpa PostgreSQL: set `DB_CONNECTION=sqlite` dan `DB_DATABASE=delcom_blog.db`.

Saat pertama dijalankan, migrasi dan seeder membuat akun **admin/admin** (dari `.env`), **member/member**, serta 7 artikel contoh (6 terbit, 1 draf).

## Pengujian

```bash
go test -v -cover ./internal/...                                   # sesuai materi
go test -v -coverprofile=coverage.out -covermode=atomic ./...      # sesuai Jenkinsfile
```

Semua paket `internal/...` dan `docs` mencapai **100.0% statement coverage**. Pengujian memakai SQLite in-memory (`glebarez/sqlite`, pure Go — aman untuk `CGO_ENABLED=0`) dan server palsu `httptest` untuk mesin pencari & OpenAI, sehingga tidak membutuhkan jaringan. Satu-satunya kode tanpa tes adalah fungsi `main()` dua baris di `main.go` dan `cmd/server/main.go`.

## Build

```bash
CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .   # sama dengan pipeline
go build -o server.exe .                                         # Windows
```

Binary membaca `.env` dari direktori kerja saat dijalankan.

## Endpoint

| Method | Endpoint | Akses |
|---|---|---|
| GET | `/swagger/*any`, `/api.json`, `/api/health` | Publik |
| GET | `/api/public/categories` | Publik |
| GET | `/api/public/posts?q=&category=` | Publik |
| GET | `/api/public/posts/:slug` | Publik |
| POST | `/api/auth/register`, `/api/auth/login`, `/api/auth/logout` | Publik |
| GET, PUT | `/api/auth/profile` | Login |
| GET, POST | `/api/posts` | Penulis |
| GET | `/api/posts/stats` | Penulis |
| POST | `/api/posts/ai-generate` | Penulis |
| GET, PUT, DELETE | `/api/posts/:id` | Penulis (pemilik) |
| PATCH | `/api/posts/:id/toggle-publish` | Penulis (pemilik) |
| GET | `/api/admin/users`, `/api/admin/posts`, `/api/admin/users/:id/posts` | Admin |

Kode status: `201` saat membuat, `400` validasi / ID tidak valid / username-email sudah terdaftar, `401` token tidak ada/tidak valid atau kredensial salah, `403` bukan admin atau artikel milik penulis lain, `404` data tidak ada, `429` rate limit (120 req/menit untuk login & register, 30 req/menit untuk AI generator).

Kontrak penting (mengikuti test suite penilaian):
- Register wajib `username`, `email`, `password` (min. 6). Field `role` opsional: hanya `admin`/`member` yang diterima, selain itu menjadi `member`. Form register di frontend tidak mengirim `role`.
- Respons artikel publik memuat `author_name` dan tidak memuat `user_id`; respons penulis dan admin memuat `user_id`.
- Statistik penulis: `total_articles`, `total_selesai` (terbit), `belum_selesai` (draf), beserta alias `total`/`published`/`draft`.
- Admin: `total_articles` per pengguna; `GET /api/admin/users/:id/posts` untuk pengguna yang tidak ada mengembalikan daftar kosong.

## Struktur

```text
main.go                       # entry point (dibangun pipeline dengan `go build .`)
cmd/server/main.go            # entry point alternatif sesuai materi
docs/                         # api.json (Swagger 2.0, di-embed), docs.go, swagger.json, swagger.yaml
internal/
├── config/                   # LoadConfig() dari .env + default
├── database/                 # model User & Post, Connect, Migrate, Seed
├── middleware/               # CORS, SecurityHeaders, Auth JWT, RequireRole, RateLimiter
├── response/                 # Success / Fail / Error + pesan validasi yang mudah dibaca
├── features/
│   ├── auth/                 # dto, repository, service, handler
│   ├── post/                 # publik + penulis + admin: dto, repository, service, handler
│   └── ai/                   # search.go, client.go, service.go, handler.go
└── server/                   # router, Bootstrap(), Run()
Jenkinsfile                   # pipeline CI/CD (test → build → Trivy → package → deploy)
```
