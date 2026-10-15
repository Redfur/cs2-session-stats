// Package ids выдаёт и проверяет идентификаторы групп, сессий, матчей и загрузок:
// префикс типа и 12 случайных символов base62. По одному id нельзя найти другой перебором.
package ids

import (
	"crypto/rand"
	"strings"
)

// Kind — тип объекта, он же префикс его id.
type Kind string

const (
	Group   Kind = "g_"
	Session Kind = "s_"
	Match   Kind = "m_"
	Import  Kind = "imp_"
)

// Length — число случайных символов после префикса (~71 бит).
const Length = 12

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// limit — байты от limit и выше отбрасываются, чтобы остаток от деления на 62 был равномерным.
const limit = 256 - 256%len(alphabet)

// New возвращает новый id типа kind из криптографически стойкого генератора.
func New(kind Kind) string {
	out := make([]byte, 0, len(kind)+Length)
	out = append(out, kind...)
	buf := make([]byte, Length*2)
	for len(out) < len(kind)+Length {
		rand.Read(buf) // crypto/rand.Read не возвращает ошибок
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == len(kind)+Length {
				break
			}
		}
	}
	return string(out)
}

// Valid сообщает, что s — id типа kind: верный префикс, длина и алфавит.
func Valid(kind Kind, s string) bool {
	rest, ok := strings.CutPrefix(s, string(kind))
	if !ok || len(rest) != Length {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if strings.IndexByte(alphabet, rest[i]) < 0 {
			return false
		}
	}
	return true
}
