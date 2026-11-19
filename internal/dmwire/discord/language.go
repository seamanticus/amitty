package discord

// Language is a string type for language codes, such as "en-US" or "fr". Refer
// to the constants for valid language codes.
//
// The list of all valid language codes are at
// https://discord.com/developers/docs/reference#locales
type Language string

const (
	Danish        Language = "da"
	German        Language = "de"
	EnglishUK     Language = "en-GB"
	EnglishUS     Language = "en-US"
	Spanish       Language = "es-ES"
	French        Language = "fr"
	Croatian      Language = "hr"
	Italian       Language = "it"
	Lithuanian    Language = "lt"
	Hungarian     Language = "hu"
	Dutch         Language = "nl"
	Norwegian     Language = "no"
	Polish        Language = "pl"
	PortugueseBR  Language = "pt-BR"
	Romanian      Language = "ro"
	Finnish       Language = "fi"
	Swedish       Language = "sv-SE"
	Vietnamese    Language = "vi"
	Turkish       Language = "tr"
	Czech         Language = "cs"
	Greek         Language = "el"
	Bulgarian     Language = "bg"
	Russian       Language = "ru"
	Ukrainian     Language = "uk"
	Hindi         Language = "hi"
	Thai          Language = "th"
	ChineseChina  Language = "zh-CN"
	Japanese      Language = "ja"
	ChineseTaiwan Language = "zh-TW"
	Korean        Language = "ko"
)
