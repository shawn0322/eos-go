package eos

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/eoscanada/eos-go/ecc"
)

// For reference:
// https://github.com/mithrilcoin-io/EosCommander/blob/master/app/src/main/java/io/mithrilcoin/eoscommander/data/remote/model/types/EosByteWriter.java

type Name string
type AccountName Name
type PermissionName Name
type ActionName Name
type TableName Name

func AN(in string) AccountName    { return AccountName(in) }
func ActN(in string) ActionName   { return ActionName(in) }
func PN(in string) PermissionName { return PermissionName(in) }

func (acct AccountName) MarshalBinary() ([]byte, error) {
	return Name(acct).MarshalBinary()
}
func (acct PermissionName) MarshalBinary() ([]byte, error) {
	return Name(acct).MarshalBinary()
}
func (acct ActionName) MarshalBinary() ([]byte, error) {
	return Name(acct).MarshalBinary()
}
func (acct TableName) MarshalBinary() ([]byte, error) {
	return Name(acct).MarshalBinary()
}
func (acct Name) MarshalBinary() ([]byte, error) {
	val, err := StringToName(string(acct))
	if err != nil {
		return nil, err
	}
	var out [8]byte
	binary.LittleEndian.PutUint64(out[:8], val)
	return out[:], nil
}

func (n *AccountName) UnmarshalBinary(data []byte) error {
	*n = AccountName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *Name) UnmarshalBinary(data []byte) error {
	*n = Name(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *PermissionName) UnmarshalBinary(data []byte) error {
	*n = PermissionName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *ActionName) UnmarshalBinary(data []byte) error {
	*n = ActionName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}
func (n *TableName) UnmarshalBinary(data []byte) error {
	*n = TableName(NameToString(binary.LittleEndian.Uint64(data)))
	return nil
}

func (AccountName) UnmarshalBinarySize() int    { return 8 }
func (PermissionName) UnmarshalBinarySize() int { return 8 }
func (ActionName) UnmarshalBinarySize() int     { return 8 }
func (TableName) UnmarshalBinarySize() int      { return 8 }
func (Name) UnmarshalBinarySize() int           { return 8 }

// OTHER TYPES: eosjs/src/structs.js

// Compression

type CompressionType uint8

const (
	CompressionNone = CompressionType(iota)
	CompressionZlib
)

func (c CompressionType) String() string {
	switch c {
	case CompressionNone:
		return "none"
	case CompressionZlib:
		return "zlib"
	default:
		return ""
	}
}

func (c CompressionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *CompressionType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "zlib":
		*c = CompressionZlib
	default:
		*c = CompressionNone
	}
	return nil
}

// CurrencyName

type CurrencyName string

func (c CurrencyName) MarshalBinary() ([]byte, error) {
	out := make([]byte, 7, 7)
	copy(out, []byte(c))
	return out, nil
}

func (c *CurrencyName) UnmarshalBinary(data []byte) error {
	*c = CurrencyName(strings.TrimRight(string(data), "\x00"))
	return nil
}
func (CurrencyName) UnmarshalBinarySize() int { return 7 }

// Asset

// NOTE: there's also ExtendedAsset which is a quantity with the attached contract (AccountName)
type Asset struct {
	Amount int64
	Symbol
}

// NOTE: there's also a new ExtendedSymbol (which includes the contract (as AccountName) on which it is)
type Symbol struct {
	Precision uint8
	Symbol    string
}

// EOSSymbol represents the standard EOS symbol on the chain.  It's
// here just to speed up things.
var EOSSymbol = Symbol{Precision: 4, Symbol: "EOS"}

func NewEOSAssetFromString(amount string) (out Asset, err error) {
	val, err := strconv.ParseInt(strings.Replace(amount, ".", "", 1), 10, 64)
	if err != nil {
		return out, err
	}
	return NewEOSAsset(val), nil
}

func NewEOSAsset(amount int64) Asset {
	return Asset{Amount: amount, Symbol: EOSSymbol}
}

// NewAsset parses a string like `1000.0000 EOS` into a properly setup Asset
func NewAsset(in string) (out Asset, err error) {
	sec := strings.SplitN(in, " ", 2)
	if len(sec) != 2 {
		return out, fmt.Errorf("invalid format %q, expected an amount and a currency symbol", in)
	}

	if len(sec[1]) > 7 {
		return out, fmt.Errorf("currency symbol %q too long", sec[1])
	}

	out.Symbol.Symbol = sec[1]
	amount := sec[0]
	amountSec := strings.SplitN(amount, ".", 2)

	if len(amountSec) == 2 {
		out.Symbol.Precision = uint8(len(amountSec[1]))
	}

	val, err := strconv.ParseInt(strings.Replace(amount, ".", "", 1), 10, 64)
	if err != nil {
		return out, err
	}

	out.Amount = val

	return
}

func (a *Asset) UnmarshalBinary(data []byte) error {
	newAsset := Asset{}
	if err := UnmarshalBinary(data[:8], &newAsset.Amount); err != nil {
		return err
	}
	if err := UnmarshalBinary(data[8:9], &newAsset.Precision); err != nil {
		return err
	}
	newAsset.Symbol.Symbol = strings.Trim(string(data[9:16]), "\x00")

	*a = newAsset

	return nil
}

func (Asset) UnmarshalBinarySize() int {
	return 16
}

func (a Asset) MarshalBinary() ([]byte, error) {
	binAsset := struct {
		Amount    int64
		Precision uint8
		Symbol    [7]byte
	}{Amount: a.Amount, Precision: a.Precision, Symbol: [7]byte{}}
	copy(binAsset.Symbol[:], []byte(a.Symbol.Symbol))
	return MarshalBinary(binAsset)
}

func (a *Asset) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	asset, err := NewAsset(s)
	if err != nil {
		return err
	}

	*a = asset

	return nil
}

type Permission struct {
	PermName     string    `json:"perm_name"`
	Parent       string    `json:"parent"`
	RequiredAuth Authority `json:"required_auth"`
}

type PermissionLevel struct {
	Actor      AccountName    `json:"actor"`
	Permission PermissionName `json:"permission"`
}

type PermissionLevelWeight struct {
	Permission PermissionLevel `json:"permission"`
	Weight     uint16          `json:"weight"`
}

type Authority struct {
	Threshold uint32                  `json:"threshold"`
	Keys      []KeyWeight             `json:"keys"`
	Accounts  []PermissionLevelWeight `json:"accounts"`
}

type KeyWeight struct {
	PublicKey ecc.PublicKey `json:"public_key"`
	Weight    uint16        `json:"weight"`
}

type Code struct {
	AccountName AccountName `json:"account_name"`
	CodeHash    string      `json:"code_hash"`
	WAST        string      `json:"wast"` // TODO: decode into Go ast, see https://github.com/go-interpreter/wagon
	ABI         ABI         `json:"abi"`
}

// JSONTime

type JSONTime struct {
	time.Time
}

const JSONTimeFormat = "2006-01-02T15:04:05"

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.Format(JSONTimeFormat))), nil
}

func (t *JSONTime) UnmarshalJSON(data []byte) (err error) {
	if string(data) == "null" {
		return nil
	}

	t.Time, err = time.Parse(`"`+JSONTimeFormat+`"`, string(data))
	return err
}

func (t *JSONTime) UnmarshalBinary(data []byte) error {
	t.Time = time.Unix(int64(binary.LittleEndian.Uint32(data)), 0).UTC()
	return nil
}

func (t JSONTime) MarshalBinary() ([]byte, error) {
	out := []byte{0, 0, 0, 0}
	binary.LittleEndian.PutUint32(out, uint32(t.Unix()))
	return out, nil
}

func (t JSONTime) UnmarshalBinarySize() int { return 4 }

// HexBytes

type HexBytes []byte

func (t HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(t))
}

func (t *HexBytes) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*t, err = hex.DecodeString(s)
	return
}

// SHA256Bytes

type SHA256Bytes []byte // should always be 32 bytes

func (t SHA256Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(t))
}

func (t *SHA256Bytes) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*t, err = hex.DecodeString(s)
	return
}

// TODO: SHA256Bytes, implement the Binary encoder... fixed size.

type Varuint32 uint32

func (a Varuint32) MarshalBinary() ([]byte, error) {
	data := make([]byte, 8, 8)
	l := binary.PutUvarint(data, uint64(a))
	//fmt.Println("VARUINT MARSHAL", a, data, data[:l])
	return data[:l], nil
}

func (a *Varuint32) UnmarshalBinaryRead(r io.Reader) error {
	size, err := binary.ReadUvarint(&ByteReader{r})
	if err != nil {
		return err
	}
	*a = Varuint32(size)
	return nil
}

// Tstamp

type Tstamp struct {
	time.Time
}

func (t Tstamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%d", t.UnixNano()))
}

func (t *Tstamp) UnmarshalJSON(data []byte) (err error) {
	var unixNano int64
	if data[0] == '"' {
		var s string
		if err = json.Unmarshal(data, &s); err != nil {
			return
		}

		unixNano, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}

	} else {
		unixNano, err = strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return err
		}
	}
	seconds := unixNano / 1e9
	nanoSecs := unixNano*1e9 - seconds
	*t = Tstamp{time.Unix(seconds, nanoSecs)}

	return nil
}

func (t *Tstamp) UnmarshalBinary(data []byte) error {
	unixNano := int64(binary.LittleEndian.Uint64(data))
	seconds := unixNano / 1e9
	nanoSecs := unixNano*1e9 - seconds
	t.Time = time.Unix(seconds, nanoSecs).UTC()
	return nil
}

func (t Tstamp) UnmarshalBinarySize() int { return 8 }

func (t Tstamp) MarshalBinary() ([]byte, error) {
	out := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint64(out, uint64(t.UnixNano()))
	return out, nil
}
