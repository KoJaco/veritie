package fastparser

func ToInt64(ids []int) []int64 {
	out := make([]int64, len(ids))

	for i, v := range ids {
		out[i] = int64(v)
	}
	return out

}
