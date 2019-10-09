package odocrypt

import (
	"bytes"
	"encoding/hex"
	"testing"
)

var provenSolutions = []struct {
	height          int
	hash            string
	workHeader      string
	offset          int
	submittedHeader string
	intensity       int
	target          []byte
}{
	{
		height:          000000,
		hash:            "00000000000006418b86014ff54b457f52665b428d5af57e80b0b7ec84c706e5",
		workHeader:      "00000020039949A3D2755F68EB1BE7F06BC471E06DC1D0099D0AF7FB00307EB700000000D1F1022F58EF3BF019472955CB5AB02853CCB66DBDD2A77164DC46790EC31129106B2A5AB34E301B00000000",
		offset:          0,
		submittedHeader: "00000020039949A3D2755F68EB1BE7F06BC471E06DC1D0099D0AF7FB00307EB700000000D1F1022F58EF3BF019472955CB5AB02853CCB66DBDD2A77164DC46790EC31129106B2A5AB34E301BC6B1D5A6",
		intensity:       28,
	},
	// {
	// 	height:          57653,
	// 	hash:            "00000000000001ccac64b49a9ebc69c6046a93f4d32d8f8f6967c8f487ed8cec",
	// 	workHeader:      []byte{0, 0, 0, 0, 0, 0, 6, 72, 174, 217, 105, 206, 174, 59, 150, 117, 251, 55, 209, 192, 241, 37, 35, 184, 2, 194, 253, 173, 207, 249, 114, 1, 62, 26, 0, 0, 0, 0, 0, 0, 41, 7, 115, 87, 0, 0, 0, 0, 56, 56, 181, 217, 76, 24, 251, 231, 137, 4, 166, 20, 40, 53, 77, 36, 148, 23, 138, 146, 2, 199, 168, 122, 71, 162, 44, 150, 144, 2, 198, 67},
	// 	offset:          805306368,
	// 	submittedHeader: []byte{0, 0, 0, 0, 0, 0, 6, 72, 174, 217, 105, 206, 174, 59, 150, 117, 251, 55, 209, 192, 241, 37, 35, 184, 2, 194, 253, 173, 207, 249, 114, 1, 7, 235, 26, 63, 0, 0, 0, 0, 41, 7, 115, 87, 0, 0, 0, 0, 56, 56, 181, 217, 76, 24, 251, 231, 137, 4, 166, 20, 40, 53, 77, 36, 148, 23, 138, 146, 2, 199, 168, 122, 71, 162, 44, 150, 144, 2, 198, 67},
	// 	intensity:       28,
	// },
}

func newSubmittedHeaderValidator(capacity int) (v *submittedHeaderValidator) {
	v = &submittedHeaderValidator{}
	v.submittedHeaders = make(chan []byte, capacity)
	return
}

type submittedHeaderValidator struct {
	submittedHeaders chan []byte
}

//SubmitHeader stores solved so they can later be validated after the testrun
func (v *submittedHeaderValidator) SubmitHeader(header []byte, job interface{}) (err error) {
	v.submittedHeaders <- header
	return
}

func (v *submittedHeaderValidator) validate(t *testing.T) {
	if len(v.submittedHeaders) != len(provenSolutions) {
		t.Fatal("Wrong number of headers reported")
	}
	for _, provenSolution := range provenSolutions {
		submittedHeader := <-v.submittedHeaders
		provenSubmitHeader, _ := hex.DecodeString(provenSolution.submittedHeader)
		if !bytes.Equal(submittedHeader, provenSubmitHeader) {
			t.Error("Mismatch\nExpected header: ", provenSolution.submittedHeader, "\nSubmitted header: ", submittedHeader)
		}
	}
}
