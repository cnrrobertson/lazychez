// Derived from github.com/jesseduffield/lazycore (MIT License, Copyright 2022 Jesse Duffield).
// samber/lo and lazycore/utils imports replaced with vanilla Go and stdlib builtins.
package boxlayout

// Dimensions holds the absolute terminal coordinates of a panel.
type Dimensions struct {
	X0 int
	X1 int
	Y0 int
	Y1 int
}

// Direction controls how a Box lays out its children.
type Direction int

const (
	// ROW stacks children vertically (each child forms a horizontal row).
	ROW Direction = iota
	// COLUMN places children side by side horizontally.
	COLUMN
)

// Box is a node in the layout tree. Leaf nodes (Window != "") map to named
// gocui views. Interior nodes split their assigned space among Children.
//
// Space allocation: static (Size > 0) children are satisfied first; the
// remaining space is shared proportionally by Weight among dynamic children.
// You must set either Size or Weight, not both.
type Box struct {
	Direction            Direction
	ConditionalDirection func(width, height int) Direction
	Children             []*Box
	ConditionalChildren  func(width, height int) []*Box
	Window               string
	Size                 int // static size in the parent's split axis
	Weight               int // dynamic weight once static children are satisfied
}

// ArrangeWindows recursively computes terminal coordinates for every named
// window in the tree rooted at root, given the available rectangle.
func ArrangeWindows(root *Box, x0, y0, width, height int) map[string]Dimensions {
	children := root.getChildren(width, height)
	if len(children) == 0 {
		if root.Window != "" {
			return map[string]Dimensions{
				root.Window: {X0: x0, Y0: y0, X1: x0 + width - 1, Y1: y0 + height - 1},
			}
		}
		return map[string]Dimensions{}
	}

	direction := root.getDirection(width, height)

	var availableSize int
	if direction == COLUMN {
		availableSize = width
	} else {
		availableSize = height
	}

	sizes := calcSizes(children, availableSize)

	result := map[string]Dimensions{}
	offset := 0
	for i, child := range children {
		boxSize := sizes[i]
		var childResult map[string]Dimensions
		if direction == COLUMN {
			childResult = ArrangeWindows(child, x0+offset, y0, boxSize, height)
		} else {
			childResult = ArrangeWindows(child, x0, y0+offset, width, boxSize)
		}
		result = mergeDimensionMaps(result, childResult)
		offset += boxSize
	}
	return result
}

func calcSizes(boxes []*Box, availableSpace int) []int {
	weights := extractWeights(boxes)
	normalizedWeights := normalizeWeights(weights)

	totalWeight := 0
	reservedSpace := 0
	for i, box := range boxes {
		if box.isStatic() {
			reservedSpace += box.Size
		} else {
			totalWeight += normalizedWeights[i]
		}
	}

	dynamicSpace := max(0, availableSpace-reservedSpace)

	unitSize := 0
	extraSpace := 0
	if totalWeight > 0 {
		unitSize = dynamicSpace / totalWeight
		extraSpace = dynamicSpace % totalWeight
	}

	result := make([]int, len(boxes))
	for i, box := range boxes {
		if box.isStatic() {
			result[i] = min(availableSpace, box.Size)
		} else {
			result[i] = unitSize * normalizedWeights[i]
		}
	}

	// Distribute remainder one unit at a time across dynamic boxes.
	for extraSpace > 0 {
		for i, weight := range normalizedWeights {
			if weight > 0 {
				result[i]++
				extraSpace--
				normalizedWeights[i]--
				if extraSpace == 0 {
					break
				}
			}
		}
	}

	return result
}

// normalizeWeights removes common multiples, e.g. [2,4,4] → [1,2,2].
func normalizeWeights(weights []int) []int {
	if len(weights) == 0 {
		return []int{}
	}
	if anyEqualsOne(weights) {
		return weights
	}

	positiveWeights := filterPositive(weights)
	if len(positiveWeights) == 0 {
		return weights
	}

	factorSlices := mapFactors(positiveWeights)
	commonFactors := factorSlices[0]
	for _, factors := range factorSlices[1:] {
		commonFactors = intersect(commonFactors, factors)
	}

	if len(commonFactors) == 0 {
		return weights
	}

	return normalizeWeights(divideWeights(weights, commonFactors[0]))
}

func calcFactors(n int) []int {
	factors := []int{}
	for i := 2; i <= n; i++ {
		if n%i == 0 {
			factors = append(factors, i)
		}
	}
	return factors
}

// --- vanilla Go replacements for samber/lo helpers ---

func extractWeights(boxes []*Box) []int {
	w := make([]int, len(boxes))
	for i, b := range boxes {
		w[i] = b.Weight
	}
	return w
}

func anyEqualsOne(weights []int) bool {
	for _, w := range weights {
		if w == 1 {
			return true
		}
	}
	return false
}

func filterPositive(weights []int) []int {
	result := make([]int, 0, len(weights))
	for _, w := range weights {
		if w > 0 {
			result = append(result, w)
		}
	}
	return result
}

func mapFactors(weights []int) [][]int {
	result := make([][]int, len(weights))
	for i, w := range weights {
		result[i] = calcFactors(w)
	}
	return result
}

func intersect(a, b []int) []int {
	bSet := make(map[int]bool, len(b))
	for _, v := range b {
		bSet[v] = true
	}
	result := make([]int, 0)
	for _, v := range a {
		if bSet[v] {
			result = append(result, v)
		}
	}
	return result
}

func divideWeights(weights []int, factor int) []int {
	result := make([]int, len(weights))
	for i, w := range weights {
		result[i] = w / factor
	}
	return result
}

// --- Box methods ---

func (b *Box) isStatic() bool {
	return b.Size > 0
}

func (b *Box) getDirection(width, height int) Direction {
	if b.ConditionalDirection != nil {
		return b.ConditionalDirection(width, height)
	}
	return b.Direction
}

func (b *Box) getChildren(width, height int) []*Box {
	if b.ConditionalChildren != nil {
		return b.ConditionalChildren(width, height)
	}
	return b.Children
}

func mergeDimensionMaps(a, b map[string]Dimensions) map[string]Dimensions {
	result := make(map[string]Dimensions, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
