package producers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-api/protocol"
	s "github.com/iotexproject/iotex-api/sql"
)

// ProtocolID is the ID of protocol
const ProtocolID = "producers"

type (
	// ProductivityHistory defines the schema of "productivity history" table
	ProductivityHistory struct {
		EpochNumber      string
		BlockHeight      string
		Producer         string
		ExpectedProducer string
	}

	// BlockProducersHistory defines the schema of "block producers history" table
	BlockProducersHistory struct {
		EpochNumber       string
		BlockProducerList []byte
	}
)

// Protocol defines the protocol of indexing blocks
type Protocol struct {
	Store                 s.Store
	NumDelegates          uint64
	NumCandidateDelegates uint64
	NumSubEpochs          uint64
}

// NewProtocol creates a new protocol
func NewProtocol(store s.Store, numDelegates uint64, numCandidateDelegates uint64, numSubEpochs uint64) *Protocol {
	return &Protocol{
		Store:                 store,
		NumDelegates:          numDelegates,
		NumCandidateDelegates: numCandidateDelegates,
		NumSubEpochs:          numSubEpochs,
	}
}

// CreateTables creates tables
func (p *Protocol) CreateTables(ctx context.Context) error {
	// create productivity history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [block_height] TEXT NOT NULL, [producer] TEXT NOT NULL, "+
		"[expected_producer] TEXT NOT NULL)",
		p.getProductivityHistoryTableName())); err != nil {
		return err
	}

	// create block producers history table
	if _, err := p.Store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s "+
		"([epoch_number] TEXT NOT NULL, [block_producer_list] BLOB(32) NOT NULL)",
		p.getBlockProducersHistoryTableName())); err != nil {
		return err
	}
	return nil
}

// HandleBlock handles blocks
func (p *Protocol) HandleBlock(ctx context.Context, tx *sql.Tx, blk *block.Block) error {
	for _, selp := range blk.Actions {
		if putPollResult, ok := selp.Action().(*action.PutPollResult); ok {
			epochNumber := protocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, putPollResult.Height())
			candidateList := putPollResult.Candidates()
			if len(candidateList) > int(p.NumCandidateDelegates) {
				candidateList = candidateList[:p.NumCandidateDelegates]
			}
			if err := p.updateBlockProducersHistory(tx, epochNumber, candidateList); err != nil {
				return errors.Wrap(err, "failed to update epoch number to block producers history table")
			}
		}
	}

	epochNum := protocol.GetEpochNumber(p.NumDelegates, p.NumSubEpochs, blk.Height())
	if err := p.updateProductivityHistory(tx, epochNum, blk.Height(), blk.ProducerAddress()); err != nil {
		return errors.Wrapf(err, "failed to update epoch number to productivity history table")
	}

	return nil
}

// ReadTable reads indices in the table
func (p *Protocol) ReadTable(context.Context, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

// updateBlockProducersHistory stores block producers information into block producers history table
func (p *Protocol) updateBlockProducersHistory(tx *sql.Tx, epochNum uint64, blockProducerList state.CandidateList) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number, block_producer_list) VALUES (?, ?)",
		p.getBlockProducersHistoryTableName())
	epochNumber := strconv.Itoa(int(epochNum))
	blockProducers, err := blockProducerList.Serialize()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(insertQuery, epochNumber, blockProducers); err != nil {
		return err
	}
	return nil
}

// updateProductivityHistory stores block producers' productivity information into productivity history table
func (p *Protocol) updateProductivityHistory(tx *sql.Tx, epochNum uint64, blockHeight uint64, blockProducer string) error {
	blockProducerList, err := p.getBlockProducersHistory(epochNum)
	if err != nil {
		return err
	}
	blockProducerAddrs := make([]string, 0)
	for _, delegate := range blockProducerList {
		blockProducerAddrs = append(blockProducerAddrs, delegate.Address)
	}
	crypto.SortCandidates(blockProducerAddrs, epochNum, crypto.CryptoSeed)
	activeProducers := blockProducerAddrs
	if len(activeProducers) > int(p.NumDelegates) {
		activeProducers = activeProducers[:p.NumDelegates]
	}
	expectedProducer := activeProducers[int(blockHeight)%len(activeProducers)]

	insertQuery := fmt.Sprintf("INSERT INTO %s (epoch_Number, block_height, producer, expected_producer) "+
		"VALUES (?, ?, ?, ?)", p.getProductivityHistoryTableName())
	epochNumber := strconv.Itoa(int(epochNum))
	height := strconv.Itoa(int(blockHeight))

	if _, err := tx.Exec(insertQuery, epochNumber, height, blockProducer, expectedProducer); err != nil {
		return err
	}
	return nil
}

// GetBlockProducersHistory returns block producers information by epoch number
func (p *Protocol) GetBlockProducersHistory(epochNumber uint64) (state.CandidateList, error) {
	return p.getBlockProducersHistory(epochNumber)
}

// GetProductivityHistory returns productivity information by epoch number and user address
func (p *Protocol) GetProductivityHistory(epochNumber uint64, address string) (uint64, uint64, error) {
	db := p.Store.GetDB()

	epochNumStr := strconv.Itoa(int(epochNumber))

	getProductionQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE epoch_number=? AND producer=?",
		p.getProductivityHistoryTableName())
	stmt, err := db.Prepare(getProductionQuery)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to prepare get query")
	}

	var productions uint64
	err = stmt.QueryRow(epochNumStr, address).Scan(&productions)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to execute get query")
	}

	getExpectedProductionQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE epoch_number=? AND expected_producer=?",
		p.getProductivityHistoryTableName())
	stmt, err = db.Prepare(getExpectedProductionQuery)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to prepare get query")
	}

	var expectedProductions uint64
	err = stmt.QueryRow(epochNumStr, address).Scan(&expectedProductions)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to execute get query")
	}

	return productions, expectedProductions, nil
}

func (p *Protocol) getBlockProducersHistory(epochNumber uint64) (state.CandidateList, error) {
	db := p.Store.GetDB()

	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE epoch_number=?",
		p.getBlockProducersHistoryTableName())
	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare get query")
	}

	epochNumStr := strconv.Itoa(int(epochNumber))
	rows, err := stmt.Query(epochNumStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var blockProducersHistory BlockProducersHistory
	parsedRows, err := s.ParseSQLRows(rows, &blockProducersHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return nil, protocol.ErrNotExist
	}

	if len(parsedRows) > 1 {
		return nil, errors.New("Only one row is expected")
	}

	blockProducers := parsedRows[0].(*BlockProducersHistory)
	var blockProducerList state.CandidateList
	if err := blockProducerList.Deserialize(blockProducers.BlockProducerList); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block producer list")
	}

	return blockProducerList, nil
}

func (p *Protocol) getProductivityHistoryTableName() string {
	return fmt.Sprint("productivity_history")
}

func (p *Protocol) getBlockProducersHistoryTableName() string {
	return fmt.Sprint("block_producers_history")
}
