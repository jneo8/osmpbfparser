package osmpbfparser

import (
	"github.com/jneo8/osmpbfparser-go/bitmask"
)

// New ...
func New(
	args Args,
) PBFParser {
	return &pbfParser{Args: args}
}

func newPBFIndexer(
	pbfFile string,
	pbfMasks *bitmask.PBFMasks,
) pbfDataParser {
	return &PBFIndexer{
		PBFFile:  pbfFile,
		PBFMasks: pbfMasks,
	}
}
