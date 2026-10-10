package flora

import (
	"math"
	"runtime"
	"sync"
)

// spread blurs a grid of seed sources with a Gaussian kernel of some cells of spread: the seeds
// that land on every cell. The kernel is separable, so it runs along the rows and then along the
// columns; edges are clamped.
func spread(sources []float32, columns, rows int, cells float64) []float32 {
	if cells < 0.3 {
		return sources
	}
	weights := gaussianWeights(cells)
	across := make([]float32, len(sources))
	inParallel(rows, func(row int) {
		for column := range columns {
			across[row*columns+column] = weightedSum(sources, weights, row*columns, column, columns, 1)
		}
	})
	result := make([]float32, len(sources))
	inParallel(rows, func(row int) {
		for column := range columns {
			result[row*columns+column] = weightedSum(across, weights, column, row, rows, columns)
		}
	})
	return result
}

// weightedSum adds the values around a position along one direction (a row or a column), clamping
// at the edges: start is the index of the first value of the line and stride the step between
// values along it.
func weightedSum(values []float32, weights []float32, start, position, count, stride int) float32 {
	reach := len(weights) / 2
	var sum float32
	for offset := -reach; offset <= reach; offset++ {
		along := min(max(position+offset, 0), count-1)
		sum += weights[offset+reach] * values[start+along*stride]
	}
	return sum
}

func gaussianWeights(sigma float64) []float32 {
	reach := int(math.Ceil(3 * sigma))
	weights := make([]float32, 2*reach+1)
	total := 0.0
	for offset := -reach; offset <= reach; offset++ {
		weight := math.Exp(-float64(offset*offset) / (2 * sigma * sigma))
		weights[offset+reach] = float32(weight)
		total += weight
	}
	for index := range weights {
		weights[index] /= float32(total)
	}
	return weights
}

// inParallel runs work for every item from 0 to count, spread over the processors. Each item must
// only write its own part of the results.
func inParallel(count int, work func(item int)) {
	workers := min(runtime.NumCPU(), count)
	if workers <= 1 {
		for item := range count {
			work(item)
		}
		return
	}
	var group sync.WaitGroup
	chunk := (count + workers - 1) / workers
	for first := 0; first < count; first += chunk {
		last := min(first+chunk, count)
		group.Add(1)
		go func() {
			defer group.Done()
			for item := first; item < last; item++ {
				work(item)
			}
		}()
	}
	group.Wait()
}
