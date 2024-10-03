# Maintainer: zvdy <zzvdyy@gmail.com>
pkgname=parsero-go
pkgver=r4.de2e324
pkgrel=1
pkgdesc="A script written in Golang to read Robots.txt files and check Disallow entries."
arch=('x86_64')
url="https://github.com/zvdy/parsero-go"
license=('MIT')
depends=('go')
source=("$pkgname-$pkgver.tar.gz::https://github.com/zvdy/parsero-go/archive/main.tar.gz")
sha256sums=('SKIP')

pkgver() {
  cd "$srcdir/$pkgname-main"
  # Use the latest commit hash as the version
  echo "r$(git rev-list --count HEAD).$(git rev-parse --short HEAD)"
}

build() {
  cd "$srcdir/$pkgname-main"
  go build -o "$srcdir/parsero" ./cmd
}

package() {
  cd "$srcdir/$pkgname-main"
  install -Dm755 "$srcdir/parsero" "$pkgdir/usr/bin/parsero"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}

# vim:set ts=2 sw=2 et: