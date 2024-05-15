package help

func XorArrays(arr [][]byte) []byte {
	// Check if the array is empty
	if len(arr) == 0 {
		return nil
	}

	length := len(arr[0])
	if length > 20 {
		length = 20
	}

	for i, slice := range arr {
		if len(slice) < 20 {
			paddedSlice := make([]byte, 20)
			copy(paddedSlice, slice)
			arr[i] = paddedSlice
		}
	}
	// Create a result slice with the same length as the first slice
	result := make([]byte, length)

	// XOR each byte in the first 20 characters of each slice
	for _, slice := range arr {
		for i := 0; i < length; i++ {
			result[i] ^= slice[i]
		}
	}

	return result
}
