package npu

import (
	"fmt"
	"math"
)

// QuantMode is the quantization strategy.
type QuantMode string

const (
	QuantNone   QuantMode = "none"
	QuantInt8   QuantMode = "int8"   // per-channel / per-tensor INT8
	QuantInt4   QuantMode = "int4"   // weight-only INT4
	QuantMixed  QuantMode = "mixed"  // mixed precision dispatch
)

// QuantScale describes per-channel scales for a tensor.
type QuantScale struct {
	Count  int
	Scale  []float64
	Zero   []float64
}

// QuantTensor is a quantized tensor operation result.
type QuantTensor struct {
	Mode     QuantMode
	Elems    int
	Data     []int8   // INT8 payload (bitpacked for int4)
	PerByte  int      // bytes per element (1 for int8, 0.5 for int4 via packing)
	Scale    QuantScale
}

// quantizeInt8 quantizes floats to INT8 with per-tensor scales.
func quantizeInt8(values []float64) QuantTensor {
	n := len(values)
	out := make([]int8, n)
	var scale float64
	var maxAbs float64
	for _, v := range values {
		if a := math.Abs(v); a > maxAbs {
			maxAbs = a
		}
	}
	if maxAbs == 0 {
		scale = 1.0
	} else {
		scale = maxAbs / 127.0
	}
	for i, v := range values {
		out[i] = int8(math.Round(v / scale))
	}
	return QuantTensor{
		Mode:    QuantInt8,
		Elems:   n,
		Data:    out,
		PerByte: 1,
		Scale:   QuantScale{Count: 1, Scale: []float64{scale}, Zero: []float64{0}},
	}
}

// quantizeInt8PerChannel quantizes a weight matrix of shape [rows, cols] (row-major)
// with a distinct per-row scale (the common inference layout).
func quantizeInt8PerChannel(values []float64, rows, cols int) QuantTensor {
	out := make([]int8, len(values))
	scales := make([]float64, rows)
	zeros := make([]float64, rows)
	for r := 0; r < rows; r++ {
		var maxAbs float64
		for c := 0; c < cols; c++ {
			if a := math.Abs(values[r*cols+c]); a > maxAbs {
				maxAbs = a
			}
		}
		sc := 1.0
		if maxAbs != 0 {
			sc = maxAbs / 127.0
		}
		scales[r] = sc
		for c := 0; c < cols; c++ {
			out[r*cols+c] = int8(math.Round(values[r*cols+c] / sc))
		}
	}
	return QuantTensor{
		Mode:    QuantInt8,
		Elems:   len(values),
		Data:    out,
		PerByte: 1,
		Scale:   QuantScale{Count: rows, Scale: scales, Zero: zeros},
	}
}

// quantizeInt4 packs values as 4-bit (weight-only), 2 per byte.
func quantizeInt4(values []float64) QuantTensor {
	n := len(values)
	packed := make([]int8, (n+1)/2)
	var scale float64
	var maxAbs float64
	for _, v := range values {
		if a := math.Abs(v); a > maxAbs {
			maxAbs = a
		}
	}
	if maxAbs == 0 {
		scale = 1.0
	} else {
		// INT4 symmetric range is [-8, 7]; use 8.0 for headroom.
		scale = maxAbs / 7.0
	}
	for i, v := range values {
		q := int8(math.Round(v / scale))
		if q > 7 {
			q = 7
		}
		if q < -8 {
			q = -8
		}
		if i%2 == 0 {
			packed[i/2] = q // low nibble
		} else {
			packed[i/2] |= q << 4
		}
	}
	return QuantTensor{
		Mode:    QuantInt4,
		Elems:   n,
		Data:    packed,
		PerByte: 0, // packed 2-per-byte; flagged by Mode
		Scale:   QuantScale{Count: 1, Scale: []float64{scale}, Zero: []float64{0}},
	}
}

// Quantizer selects and applies a quantization mode.
type Quantizer struct {
	Mode      QuantMode
	PerChannel bool
}

// NewQuantizer creates a quantizer with the given mode.
func NewQuantizer(mode QuantMode) *Quantizer {
	return &Quantizer{Mode: mode}
}

// Quantize applies the configured mode to a float tensor.
// For per-channel INT8, values are treated as [rows, cols].
func (q *Quantizer) Quantize(values []float64, rows, cols int) (QuantTensor, error) {
	switch q.Mode {
	case QuantNone:
		return QuantTensor{Mode: QuantNone, Elems: len(values)}, nil
	case QuantInt8:
		if q.PerChannel && rows > 0 && cols > 0 && len(values) == rows*cols {
			return quantizeInt8PerChannel(values, rows, cols), nil
		}
		return quantizeInt8(values), nil
	case QuantInt4:
		return quantizeInt4(values), nil
	case QuantMixed:
		return q.quantizeMixed(values, rows, cols), nil
	default:
		return QuantTensor{}, fmt.Errorf("unknown quantization mode %q", q.Mode)
	}
}

// quantizeMixed dispatches: weights (>=2D) get INT8 per-channel; small/bias
// tensors stay in the most compact safe form (INT8 scalar scale).
func (q *Quantizer) quantizeMixed(values []float64, rows, cols int) QuantTensor {
	if rows > 0 && cols > 0 && len(values) == rows*cols && rows > 1 {
		return quantizeInt8PerChannel(values, rows, cols)
	}
	return quantizeInt8(values)
}

// Dequant returns reconstructed approximate float values for a quantized tensor.
func (q *QuantTensor) Dequant() []float64 {
	out := make([]float64, q.Elems)
	switch q.Mode {
	case QuantInt4:
		for i := 0; i < q.Elems; i++ {
			b := q.Data[i/2]
			var nib int8
			if i%2 == 0 {
				nib = b & 0x0F
			} else {
				nib = (b >> 4) & 0x0F
			}
			if nib > 7 {
				nib -= 16
			}
			var sc float64
			if q.Scale.Count == 1 {
				sc = q.Scale.Scale[0]
			} else {
				sc = q.Scale.Scale[minInt(i, q.Scale.Count-1)]
			}
			out[i] = float64(nib) * sc
		}
	default: // int8 and none
		for i := 0; i < q.Elems; i++ {
			sc := 1.0
			if q.Scale.Count > 0 {
				if q.Scale.Count == 1 {
					sc = q.Scale.Scale[0]
				} else {
					sc = q.Scale.Scale[minInt(i, q.Scale.Count-1)]
				}
			}
			if i < len(q.Data) {
				out[i] = float64(q.Data[i]) * sc
			}
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
