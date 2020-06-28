package osmpbfparser

import (
	"bytes"
	"encoding/binary"
	"github.com/jneo8/osmpbfparser-go/bitmask"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/thomersch/gosmparse"
	"math"
	"os"
	"strconv"
	"sync"
)

type pbfParser struct {
	PBFMasks *bitmask.PBFMasks

	// leveldb
	LevelDB *leveldb.DB
	Args    Args

	// Log
	Logger      *log.Logger
	elementChan chan Element
}

// Run ...
func (p *pbfParser) Run() error {
	p.Logger.Infof("%+v", p)
	db, err := leveldb.OpenFile(
		p.Args.LevelDBPath,
		&opt.Options{DisableBlockCache: true},
	)
	if err != nil {
		p.Logger.Error(err)
		return err
	}
	defer db.Close()
	p.LevelDB = db

	// Index
	indexer := newPBFIndexer(p.Args.PBFFile, p.PBFMasks)
	if err := indexer.Run(); err != nil {
		return err
	}
	// Relation member indexer
	relationMemberIndexer := newPBFRelationMemberIndexer(p.Args.PBFFile, p.PBFMasks)
	if err := relationMemberIndexer.Run(); err != nil {
		return err
	}

	p.Logger.Info("Finish index")

	reader, err := os.Open(p.Args.PBFFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	// FirstRound
	// Put way refs, relation member into db.
	batch := leveldb.MakeBatch(p.Args.FlushSize * 1024 * 1024)
	p.elementChan = make(chan Element)

	firstRoundWg := sync.WaitGroup{}
	firstRoundWg.Add(1)
	errCount := make(map[int]int)
	go func() {
		defer firstRoundWg.Done()
		for emt := range p.elementChan {
			switch emt.Type {
			case 0:
				if p.PBFMasks.WayRefs.Has(emt.Node.ID) || p.PBFMasks.RelNodes.Has(emt.Node.ID) {
					id, b := nodeToBytes(emt.Node)
					batch.Put(
						[]byte(id),
						b,
					)
				}
			case 1:
				if p.PBFMasks.Ways.Has(emt.Way.ID) {
					emtBytes, err := emt.ToBytes()
					if err != nil {
						errCount[1]++
						continue
					}
					batch.Put(
						[]byte("W"+strconv.FormatInt(emt.Way.ID, 10)),
						emtBytes,
					)
				}
			case 2:
				if p.PBFMasks.RelRelation.Has(emt.Relation.ID) {
					emtBytes, err := emt.ToBytes()
					if err != nil {
						errCount[2]++
						continue
					}
					batch.Put(
						[]byte("R"+strconv.FormatInt(emt.Relation.ID, 10)),
						emtBytes,
					)

				}
			}
		}
	}()
	firstRoundDecoder := gosmparse.NewDecoder(reader)
	if err := firstRoundDecoder.Parse(p); err != nil {
		return err
	}
	close(p.elementChan)
	firstRoundWg.Wait()
	p.Logger.Info("Finish first round")

	return nil
}

// ReadNode ...
func (p *pbfParser) ReadNode(node gosmparse.Node) {
	p.elementChan <- Element{
		Type: 0,
		Node: node,
	}
}

// ReadWay ...
func (p *pbfParser) ReadWay(way gosmparse.Way) {
	p.elementChan <- Element{
		Type: 1,
		Way:  way,
	}
}

// ReadRelation ...
func (p *pbfParser) ReadRelation(relation gosmparse.Relation) {
	p.elementChan <- Element{
		Type:     2,
		Relation: relation,
	}
}

// SetLogger ...
func (p *pbfParser) SetLogger(logger *log.Logger) {
	p.Logger = logger
}

func nodeToBytes(n gosmparse.Node) (string, []byte) {
	var buf bytes.Buffer

	var latBytes = make([]byte, 8)
	binary.BigEndian.PutUint64(latBytes, math.Float64bits(n.Lat))
	buf.Write(latBytes)
	return strconv.FormatInt(n.ID, 10), buf.Bytes()
}
