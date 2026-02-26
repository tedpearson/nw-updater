/*
Package crypto provides an AES decryptor that decrypts secrets encrypted by openssl.
To encrypt a secret to be read by the OpenSslDecryptor, use this command:

	echo -n "account_password" | openssl aes-256-cbc -a -md SHA256
*/
package crypto

import (
	"os"
	"strings"

	"github.com/Luzifer/go-openssl/v4"
)

// OpenSslDecryptor is used to decrypt strings.
type OpenSslDecryptor struct {
	passphrase string
	o          *openssl.OpenSSL
}

// NewOpenSslDecryptor creates a new OpenSslDecryptor, loading the passphrase for decrypting secrets from the file provided.
// Whitespace is stripped from the start and end of the file.
func NewOpenSslDecryptor(file string) OpenSslDecryptor {
	contents, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return OpenSslDecryptor{passphrase: strings.TrimSpace(string(contents)), o: openssl.New()}
}

// Decrypt decrypts a base64 encoded aes-256-cbc string. If there is an error decrypting, the original string is returned.
func (d OpenSslDecryptor) Decrypt(s string) string {
	decoded, err := d.o.DecryptBytes(d.passphrase, []byte(s), openssl.BytesToKeySHA256)
	if err != nil {
		return s
	}
	return string(decoded)
}
