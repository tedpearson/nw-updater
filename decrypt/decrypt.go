package decrypt

import (
	"os"
	"strings"

	"github.com/Luzifer/go-openssl/v4"
)

type Decryptor struct {
	passphrase string
	o          *openssl.OpenSSL
}

func NewDecryptor(file string) Decryptor {
	contents, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return Decryptor{passphrase: strings.TrimSpace(string(contents)), o: openssl.New()}
}

func (d Decryptor) Decrypt(s string) string {
	decoded, err := d.o.DecryptBytes(d.passphrase, []byte(s), openssl.BytesToKeySHA256)
	if err != nil {
		return s
	}
	return string(decoded)
}
