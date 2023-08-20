/*
Package decrypt provides an AES decryptor that decrypts secrets encrypted by openssl.
To encrypt a secret to be read by the Decryptor, use this command:

	echo -n "account_password" | openssl aes-256-cbc -a -md SHA256
*/
package decrypt

import (
	"os"
	"strings"

	"github.com/Luzifer/go-openssl/v4"
)

// Decryptor is used to decrypt strings.
type Decryptor struct {
	passphrase string
	o          *openssl.OpenSSL
}

// NewDecryptor creates a new Decryptor, loading the passphrase for decrypting secrets from the file provided.
// Whitespace is stripped from the start and end of the file.
func NewDecryptor(file string) Decryptor {
	contents, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return Decryptor{passphrase: strings.TrimSpace(string(contents)), o: openssl.New()}
}

// Decrypt decrypts a base64 encoded aes-256-cbc string.
func (d Decryptor) Decrypt(s string) string {
	decoded, err := d.o.DecryptBytes(d.passphrase, []byte(s), openssl.BytesToKeySHA256)
	if err != nil {
		return s
	}
	return string(decoded)
}
