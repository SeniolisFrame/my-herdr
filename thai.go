package main

// thaiKeys maps what the Thai Kedmanee layout types to the Latin key in the
// same position, so the explorer's shortcuts work without switching layout.
var thaiKeys = map[string]string{
	"ๆ": "q", "ห": "s", "แ": "c", "พ": "r", "ฟ": "a",
	"่": "j", "า": "k", "้": "h", "ส": "l",
	"เ": "g", "ฌ": "G", "ื": "n", "ย": "p", "น": "o",
	"บ": "[", "ล": "]",
	// Fork-like actions (shifted keys use the Kedmanee shift layer).
	"ฆ": "S", "๊": "U", "ฏ": "D", "ิ": "b", "ฺ": "B", "ท": "m",
	"ฯ": "O", "ธ": "T", ")": "X", "ด": "f", "โ": "F", "ญ": "P",
	"ผ": "z", "(": "Z", "ศ": "L", "ฦ": "?",
}
