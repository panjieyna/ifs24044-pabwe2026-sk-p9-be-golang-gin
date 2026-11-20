package database

import "time"

func daysAgo(now time.Time, days int) *time.Time {
	t := now.Add(-time.Duration(days) * 24 * time.Hour)
	return &t
}

// samplePosts menyiapkan artikel contoh agar galeri publik tidak kosong saat pertama dijalankan.
func samplePosts(adminID, memberID uint, now time.Time) []Post {
	return []Post{
		{
			UserID:      memberID,
			Title:       "Clean Architecture di Golang: Memisahkan Handler, Service, dan Repository",
			Slug:        "clean-architecture-di-golang-memisahkan-handler-service-dan-repository",
			Category:    "Teknologi",
			Excerpt:     "Struktur feature-first membuat kode Go mudah diuji. Berikut cara membagi tanggung jawab tiap lapisan tanpa membuat proyek terasa berat.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 1),
			Content: `Banyak proyek Go dimulai dari satu berkas ` + "`main.go`" + ` yang terus membesar. Masalahnya baru terasa ketika fitur bertambah: logika bisnis bercampur dengan query database dan parsing request.

## Tiga lapisan, tiga tanggung jawab

- **Handler** menerima request HTTP, memvalidasi input, dan menerjemahkan hasil menjadi respons JSON.
- **Service** berisi aturan bisnis: membuat slug unik, menghitung waktu baca, memeriksa kepemilikan artikel.
- **Repository** satu-satunya tempat yang berbicara dengan database melalui GORM.

Setiap lapisan bergantung pada *interface*, bukan implementasi konkret. Karena itu service dapat diuji dengan repository palsu, dan handler dapat diuji dengan service palsu.

## Feature-first, bukan layer-first

Alih-alih folder ` + "`handlers/`, `services/`, `repositories/`" + ` di tingkat atas, kelompokkan berdasarkan domain: ` + "`features/auth`, `features/post`, `features/ai`" + `. Saat membaca fitur artikel, semua yang relevan ada dalam satu folder.

## Uji dari luar ke dalam

Gunakan ` + "`httptest`" + ` dan SQLite in-memory untuk menguji alur nyata: register, login, membuat artikel, lalu membacanya lewat endpoint publik. Tes seperti ini menangkap kesalahan integrasi yang sering lolos dari unit test murni.

> Struktur yang baik bukan yang paling banyak foldernya, tetapi yang membuat perubahan berikutnya terasa aman.`,
		},
		{
			UserID:      memberID,
			Title:       "Teknik Feynman: Belajar dengan Cara Menjelaskan",
			Slug:        "teknik-feynman-belajar-dengan-cara-menjelaskan",
			Category:    "Pendidikan",
			Excerpt:     "Jika belum bisa menjelaskan sebuah konsep dengan bahasa sederhana, kemungkinan besar kita belum benar-benar memahaminya.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 3),
			Content: `Richard Feynman dikenal mampu menjelaskan fisika rumit dengan bahasa sehari-hari. Dari kebiasaannya lahir sebuah teknik belajar yang sederhana namun efektif.

## Empat langkah

1. **Pilih satu konsep** dan tulis judulnya di bagian atas kertas.
2. **Jelaskan seolah kepada siswa SMP.** Hindari istilah teknis yang tidak kamu definisikan.
3. **Temukan celah.** Bagian yang membuatmu ragu atau kembali ke catatan adalah bagian yang belum dipahami.
4. **Sederhanakan dan gunakan analogi.** Ulangi sampai penjelasanmu mengalir.

## Mengapa berhasil

Membaca ulang memberi ilusi paham karena materi terasa familier. Menjelaskan memaksa otak *mengambil kembali* informasi, dan proses pengambilan inilah yang memperkuat ingatan.

## Praktik untuk mahasiswa informatika

Setelah mempelajari middleware JWT, coba jelaskan alurnya tanpa melihat kode: dari mana token berasal, siapa yang menandatangani, dan bagaimana server tahu token itu sah. Bila tersendat, kembali ke dokumentasi, lalu jelaskan ulang.`,
		},
		{
			UserID:      memberID,
			Title:       "Digital Minimalism: Merapikan Notifikasi, Merapikan Pikiran",
			Slug:        "digital-minimalism-merapikan-notifikasi-merapikan-pikiran",
			Category:    "Gaya Hidup",
			Excerpt:     "Bukan soal meninggalkan teknologi, melainkan memilih dengan sadar alat mana yang benar-benar menambah nilai dalam hari kita.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 5),
			Content: `Rata-rata kita membuka ponsel puluhan kali sehari, sebagian besar tanpa tujuan jelas. Digital minimalism menawarkan pendekatan berbeda: gunakan teknologi secara sengaja, bukan refleks.

## Mulai dari notifikasi

Matikan semua notifikasi kecuali dari manusia yang membutuhkan responsmu segera. Notifikasi promosi, rekomendasi, dan "seseorang menyukai postinganmu" bisa dicek pada waktunya sendiri.

## Jadwalkan waktu layar

- Pagi: satu jam pertama tanpa media sosial.
- Kerja: blok fokus 50 menit, ponsel di ruangan lain.
- Malam: mode pesawat 30 menit sebelum tidur.

## Uji coba 30 hari

Hapus aplikasi yang tidak esensial selama sebulan. Setelah itu, pasang kembali hanya yang benar-benar kamu rindukan dan tetapkan aturan penggunaannya. Banyak orang terkejut betapa sedikit yang ternyata mereka butuhkan.`,
		},
		{
			UserID:      memberID,
			Title:       "Validasi Ide Usaha Sebelum Menulis Satu Baris Kode",
			Slug:        "validasi-ide-usaha-sebelum-menulis-satu-baris-kode",
			Category:    "Bisnis",
			Excerpt:     "Produk gagal lebih sering karena tidak ada yang membutuhkannya, bukan karena teknologinya buruk. Uji permintaan pasar sejak awal.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 8),
			Content: `Sebagai developer, godaan terbesar adalah langsung membangun. Padahal pertanyaan terpenting belum terjawab: apakah ada orang yang mau membayar untuk solusi ini?

## Wawancarai calon pengguna

Bicaralah dengan sepuluh orang yang mengalami masalah tersebut. Tanyakan pengalaman nyata mereka, bukan pendapat tentang ide kamu. "Kapan terakhir kali kamu mengalami ini?" jauh lebih jujur daripada "Apakah kamu akan memakai aplikasi ini?"

## Buat halaman landing

Jelaskan nilai produk dalam satu kalimat, tambahkan tombol daftar tunggu, lalu arahkan sedikit trafik. Tingkat konversi memberi sinyal awal yang terukur.

## Concierge MVP

Layani beberapa pelanggan pertama secara manual. Prosesnya tidak efisien, tetapi kamu belajar persis langkah mana yang layak diotomatisasi.

Validasi bukan berarti menunda membangun, melainkan memastikan yang dibangun memang ditunggu.`,
		},
		{
			UserID:      memberID,
			Title:       "Mengapa 100% Code Coverage Bukan Tujuan Akhir",
			Slug:        "mengapa-100-code-coverage-bukan-tujuan-akhir",
			Category:    "Opini",
			Excerpt:     "Coverage penuh membuktikan setiap baris pernah dijalankan, bukan bahwa setiap perilaku sudah benar. Keduanya berbeda.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 12),
			Content: `Angka 100% di laporan coverage terasa memuaskan. Namun angka itu hanya menjawab satu pertanyaan: apakah baris ini pernah dieksekusi saat tes berjalan?

## Yang tidak diukur coverage

- Apakah **assertion** benar-benar memeriksa hasil yang tepat.
- Apakah kasus batas, seperti input kosong atau karakter unik, sudah diuji.
- Apakah perilaku tetap benar ketika dua fitur berinteraksi.

## Tetap berguna sebagai alarm

Coverage yang turun drastis setelah sebuah pull request adalah sinyal bahwa kode baru belum diuji. Di situlah nilainya: sebagai alarm, bukan sebagai target.

## Pendekatan yang lebih sehat

Tulis tes dari sudut pandang pengguna. Untuk API blog: daftar, login, buat draf, terbitkan, lalu pastikan artikel muncul di halaman publik dengan slug yang benar. Bila tes ini lulus, coverage tinggi biasanya datang dengan sendirinya.`,
		},
		{
			UserID:      adminID,
			Title:       "Selamat Datang di Delcom Blog",
			Slug:        "selamat-datang-di-delcom-blog",
			Category:    "Umum",
			Excerpt:     "Ruang menulis untuk mahasiswa dan praktisi. Tulis dengan Markdown, simpan sebagai draf, dan terbitkan saat siap.",
			IsPublished: true,
			PublishedAt: daysAgo(now, 15),
			Content: `Delcom Blog adalah ruang menulis sederhana untuk berbagi catatan belajar, opini, dan pengalaman.

## Cara mulai menulis

1. Daftar akun penulis, lalu masuk ke **Ruang Kerja**.
2. Pilih **Tulis Artikel** dan gunakan editor Markdown dengan pratinjau langsung.
3. Simpan sebagai draf atau langsung terbitkan.

## Asisten AI dengan penelusuran web

Butuh titik awal? Gunakan **AI Generator**. Sistem menelusuri sumber terkini melalui mesin pencari, lalu menyusun draf lengkap dengan judul, ringkasan, isi, dan daftar rujukan yang bisa kamu periksa ulang.

Selamat menulis.`,
		},
		{
			UserID:   memberID,
			Title:    "Rencana Seri Tulisan: Indeks PostgreSQL untuk Pemula",
			Slug:     "rencana-seri-tulisan-indeks-postgresql-untuk-pemula",
			Category: "Teknologi",
			Excerpt:  "Draf kerangka seri tulisan tentang B-Tree, indeks komposit, dan cara membaca EXPLAIN ANALYZE.",
			Content: `## Kerangka

1. Apa itu indeks dan kapan PostgreSQL memakainya
2. B-Tree, Hash, dan GIN secara singkat
3. Indeks komposit: urutan kolom itu penting
4. Membaca EXPLAIN ANALYZE langkah demi langkah`,
		},
	}
}
