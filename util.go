package main

func unique(ids []int) []int {
	tmp := make(map[int]bool)

	for _, id := range ids {
		tmp[id] = true
	}

	var unique []int
	for k, _ := range tmp {
		unique = append(unique, k)
	}

	return unique
}
