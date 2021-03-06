package models

import (
	"context"
	"time"

	"github.com/dipdup-net/go-lib/node"
	pg "github.com/go-pg/pg/v10"
	"github.com/pkg/errors"
)

// Statuses
const (
	StatusApplied       = "applied"
	StatusBranchDelayed = "branch_delayed"
	StatusBranchRefused = "branch_refused"
	StatusRefused       = "refused"
	StatusInChain       = "in_chain"
	StatusExpired       = "expired"
)

// DefaultConstraint -
type DefaultConstraint interface {
	TransferTicket | TxRollupCommit | TxRollupDispatchTickets | TxRollupFinalizeCommitment | TxRollupOrigination |
		TxRollupRejection | TxRollupRemoveCommitment | TxRollupReturnBond | TxRollupSubmitBatch
}

// MempoolOperation -
type MempoolOperation struct {
	CreatedAt       int64   `json:"-"`
	UpdatedAt       int64   `json:"-"`
	Network         string  `json:"network" pg:",pk"`
	Hash            string  `json:"hash" pg:",pk"`
	Branch          string  `json:"branch"`
	Status          string  `json:"status"`
	Kind            string  `json:"kind"`
	Signature       string  `json:"signature"`
	Protocol        string  `json:"protocol"`
	Level           uint64  `json:"level"`
	Errors          JSONB   `json:"errors,omitempty" pg:"type:jsonb"`
	ExpirationLevel *uint64 `json:"expiration_level"`
	Raw             JSONB   `json:"raw,omitempty" pg:"type:jsonb"`
}

// BeforeInsert -
func (op *MempoolOperation) BeforeInsert(ctx context.Context) (context.Context, error) {
	op.CreatedAt = time.Now().Unix()
	op.UpdatedAt = op.CreatedAt
	return ctx, nil
}

// BeforeUpdate -
func (op *MempoolOperation) BeforeUpdate(ctx context.Context) (context.Context, error) {
	op.UpdatedAt = time.Now().Unix()
	return ctx, nil
}

func networkAndBranch(network, branch string) func(db *pg.Query) (*pg.Query, error) {
	return func(db *pg.Query) (*pg.Query, error) {
		return db.Where("network = ?", network).Where("branch = ?", branch), nil
	}
}

func isApplied(db *pg.Query) (*pg.Query, error) {
	return db.Where("status = ?", StatusApplied), nil
}

func isInChain(db *pg.Query) (*pg.Query, error) {
	return db.Where("status = ?", StatusInChain), nil
}

// SetInChain -
func SetInChain(db pg.DBI, network, hash, kind string, level uint64) error {
	model, err := getModelByKind(kind)
	if err != nil {
		return err
	}

	if _, err := db.Model(model).
		Set("status = ?, level = ?, errors = NULL", StatusInChain, level).
		Where("hash = ?", hash).
		Where("network = ?", network).
		Update(); err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}

// SetExpired -
func SetExpired(db pg.DBI, network, branch string, kinds ...string) error {
	if len(kinds) == 0 {
		return nil
	}

	for _, kind := range kinds {
		model, err := getModelByKind(kind)
		if err != nil {
			return err
		}

		if _, err := db.Model(model).Set("status = ?", StatusExpired).Apply(networkAndBranch(network, branch)).Apply(isApplied).Update(); err != nil {
			return err
		}
	}
	return nil
}

// Rollback -
func Rollback(db pg.DBI, network, branch string, level uint64, kinds ...string) error {
	if len(kinds) == 0 {
		return nil
	}

	for _, kind := range kinds {
		model, err := getModelByKind(kind)
		if err != nil {
			return err
		}

		query := db.Model(model).Apply(networkAndBranch(network, branch))

		if _, err := query.Set("status", StatusBranchRefused).
			WhereGroup(func(q *pg.Query) (*pg.Query, error) {
				return q.Where("status = ?", StatusApplied).WhereOrGroup(func(q1 *pg.Query) (*pg.Query, error) {
					return q1.Where("status = ?", StatusInChain).Where("level = ?", level), nil
				}), nil
			}).
			Update(); err != nil {
			return err
		}

		if _, err := db.Model(model).
			Set("status = ?", StatusApplied).
			Apply(networkAndBranch(network, branch)).
			Apply(isInChain).
			Where("level < ?", level).
			Update(); err != nil {
			return err
		}
	}

	return nil
}

// DeleteOldOperations -
func DeleteOldOperations(db pg.DBI, timeout uint64, status string, kinds ...string) error {
	if len(kinds) == 0 {
		return nil
	}

	for _, kind := range kinds {
		model, err := getModelByKind(kind)
		if err != nil {
			return err
		}
		ts := time.Now().Unix() - int64(timeout)
		query := db.Model(model).Where("updated_at < ?", ts)

		if status != "" {
			query.Where("status = ?", status)
		}

		if _, err := query.Delete(); err != nil {
			return err
		}
	}
	return nil
}

// GetModelsBy -
func GetModelsBy(kinds ...string) []interface{} {
	var hasManager bool
	data := make([]interface{}, 0, len(kinds))
	for i := range kinds {
		hasManager = hasManager || node.IsManager(kinds[i])
		model, err := getModelByKind(kinds[i])
		if err == nil {
			data = append(data, model)
		}
	}

	if hasManager {
		data = append(data, &GasStats{})
	}
	return data
}

func getModelByKind(kind string) (interface{}, error) {
	switch kind {
	case node.KindActivation:
		return &ActivateAccount{}, nil
	case node.KindBallot:
		return &Ballot{}, nil
	case node.KindDelegation:
		return &Delegation{}, nil
	case node.KindDoubleBaking:
		return &DoubleBaking{}, nil
	case node.KindDoubleEndorsing:
		return &DoubleEndorsing{}, nil
	case node.KindEndorsement:
		return &Endorsement{}, nil
	case node.KindNonceRevelation:
		return &NonceRevelation{}, nil
	case node.KindOrigination:
		return &Origination{}, nil
	case node.KindProposal:
		return &Proposal{}, nil
	case node.KindReveal:
		return &Reveal{}, nil
	case node.KindTransaction:
		return &Transaction{}, nil
	case node.KindRegisterGlobalConstant:
		return &RegisterGlobalConstant{}, nil
	case node.KindDoublePreendorsement:
		return &DoublePreendorsing{}, nil
	case node.KindPreendorsement:
		return &Preendorsement{}, nil
	case node.KindSetDepositsLimit:
		return &SetDepositsLimit{}, nil
	case node.KindTransferTicket:
		return &TransferTicket{}, nil
	case node.KindTxRollupCommit:
		return &TxRollupCommit{}, nil
	case node.KindTxRollupDispatchTickets:
		return &TxRollupDispatchTickets{}, nil
	case node.KindTxRollupFinalizeCommitment:
		return &TxRollupFinalizeCommitment{}, nil
	case node.KindTxRollupOrigination:
		return &TxRollupOrigination{}, nil
	case node.KindTxRollupRejection:
		return &TxRollupRejection{}, nil
	case node.KindTxRollupRemoveCommitment:
		return &TxRollupRemoveCommitment{}, nil
	case node.KindTxRollupReturnBond:
		return &TxRollupReturnBond{}, nil
	case node.KindTxRollupSubmitBatch:
		return &TxRollupSubmitBatch{}, nil

	default:
		return nil, errors.Wrap(node.ErrUnknownKind, kind)
	}

}
