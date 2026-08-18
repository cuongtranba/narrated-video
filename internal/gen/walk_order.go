package gen

func WalkOrder(nodes []string, edges [][2]string) (order []string, cycle []string) {
	if len(nodes) == 0 {
		return nil, nil
	}

	remaining := make(map[string]int, len(nodes))
	adjacency := make(map[string][]string, len(nodes))
	for _, id := range nodes {
		remaining[id] = 0
	}
	for _, edge := range edges {
		source, target := edge[0], edge[1]
		adjacency[source] = append(adjacency[source], target)
		remaining[target]++
	}

	visited := make(map[string]bool, len(nodes))
	order = make([]string, 0, len(nodes))

	for len(order) < len(nodes) {
		next := ""
		for _, id := range nodes {
			if !visited[id] && remaining[id] == 0 {
				next = id
				break
			}
		}
		if next == "" {
			break
		}
		visited[next] = true
		order = append(order, next)
		for _, target := range adjacency[next] {
			remaining[target]--
		}
	}

	if len(order) == len(nodes) {
		return order, nil
	}

	for _, id := range nodes {
		if !visited[id] {
			cycle = append(cycle, id)
		}
	}
	return append([]string{}, nodes...), cycle
}
