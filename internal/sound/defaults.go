package sound

// ParamBlockSize is the unescaped parameter block size for a FLASH (0x63) sound.
// Matches sysex.ParamBlockSizeFLASH but is redeclared here to avoid a circular import.
const ParamBlockSize = 132

// DefaultBlankParams returns a 132-byte parameter block initialised with the
// Tempest reference signature at bytes 0–15 and zeros elsewhere.
// The reference signature matches the factory-default initialisation values
// and is required by ExtractSoundsFromProject for detection (ref §3.5.2).
func DefaultBlankParams() []byte {
	p := make([]byte, ParamBlockSize)
	copy(p, referenceSignature)
	return p
}

// referenceSignature is the known 16-byte pattern at the start of a default
// Tempest parameter block (from KnobKraft reverse-engineering + ref §3.5.2).
var referenceSignature = []byte{
	0x24, 0x19, 0x00, 0x10, 0x49, 0x06, 0x00, 0x04,
	0x14, 0x1d, 0x00, 0x20, 0x50, 0x36, 0x23, 0x00,
}
