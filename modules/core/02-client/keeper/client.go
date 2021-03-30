package keeper

import (
	"encoding/hex"
	"reflect"

	"github.com/armon/go-metrics"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/ibc-go/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

// CreateClient creates a new client state and populates it with a given consensus
// state as defined in https://github.com/cosmos/ics/tree/master/spec/ics-002-client-semantics#create
func (k Keeper) CreateClient(
	ctx sdk.Context, clientState exported.ClientState, consensusState exported.ConsensusState,
) (string, error) {
	params := k.GetParams(ctx)
	if !params.IsAllowedClient(clientState.ClientType()) {
		return "", sdkerrors.Wrapf(
			types.ErrInvalidClientType,
			"client state type %s is not registered in the allowlist", clientState.ClientType(),
		)
	}

	clientID := k.GenerateClientIdentifier(ctx, clientState.ClientType())

	k.SetClientState(ctx, clientID, clientState)
	k.Logger(ctx).Info("client created at height", "client-id", clientID, "height", clientState.GetLatestHeight().String())

	// verifies initial consensus state against client state and initializes client store with any client-specific metadata
	// e.g. set ProcessedTime in Tendermint clients
	if err := clientState.Initialize(ctx, k.cdc, k.ClientStore(ctx, clientID), consensusState); err != nil {
		return "", err
	}

	// check if consensus state is nil in case the created client is Localhost
	if consensusState != nil {
		k.SetClientConsensusState(ctx, clientID, clientState.GetLatestHeight(), consensusState)
	}

	k.Logger(ctx).Info("client created at height", "client-id", clientID, "height", clientState.GetLatestHeight().String())

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"ibc", "client", "create"},
			1,
			[]metrics.Label{telemetry.NewLabel("client-type", clientState.ClientType())},
		)
	}()

	return clientID, nil
}

// UpdateClient updates the consensus state and the state root from a provided header.
func (k Keeper) UpdateClient(ctx sdk.Context, clientID string, header exported.Header) error {
	clientState, found := k.GetClientState(ctx, clientID)
	if !found {
		return sdkerrors.Wrapf(types.ErrClientNotFound, "cannot update client with ID %s", clientID)
	}

	// prevent update if the client is frozen
	if clientState.IsFrozen() {
		return sdkerrors.Wrapf(types.ErrClientFrozen, "cannot update client with ID %s", clientID)
	}

	var (
		existingConsState exported.ConsensusState
		exists            bool
		consensusHeight   exported.Height
	)
	eventType := types.EventTypeUpdateClient
	if header != nil {
		existingConsState, exists = k.GetClientConsensusState(ctx, clientID, header.GetHeight())
	}

	cacheCtx, writeFn := ctx.CacheContext()
	newClientState, newConsensusState, err := clientState.CheckHeaderAndUpdateState(cacheCtx, k.cdc, k.ClientStore(ctx, clientID), header)
	if err != nil {
		return sdkerrors.Wrapf(err, "cannot update client with ID %s", clientID)
	}

	// emit the full header in events
	var headerStr string
	if header != nil {
		// Marshal the Header as an Any and encode the resulting bytes to hex.
		// This prevents the event value from containing invalid UTF-8 characters
		// which may cause data to be lost when JSON encoding/decoding.
		headerStr = hex.EncodeToString(types.MustMarshalHeader(k.cdc, header))
		// set default consensus height with header height
		consensusHeight = header.GetHeight()

	}

	// If an existing consensus state doesn't exist, then write the update state changes,
	// and set new consensus state.
	// Else if there already exists a consensus state in client store for the header height
	// and it does not match the updated consensus state, this is evidence of misbehaviour
	// and we must freeze the client and emit appropriate events.
	if !exists {
		// write any cached state changes from CheckHeaderAndUpdateState
		// to store metadata in client store for new consensus state.
		writeFn()

		// if update is not misbehaviour then update the consensus state
		// we don't set consensus state for localhost client
		if header != nil && clientID != exported.Localhost {
			k.SetClientConsensusState(ctx, clientID, header.GetHeight(), newConsensusState)
		} else {
			consensusHeight = types.GetSelfHeight(ctx)
		}
		k.SetClientState(ctx, clientID, newClientState)

		k.Logger(ctx).Info("client state updated", "client-id", clientID, "height", consensusHeight.String())
	} else if exists && !reflect.DeepEqual(existingConsState, newConsensusState) {
		clientState.Freeze(header)
		k.SetClientState(ctx, clientID, clientState)

		// set eventType to SubmitMisbehaviour
		eventType = types.EventTypeSubmitMisbehaviour

		k.Logger(ctx).Info("client frozen due to misbehaviour", "client-id", clientID, "height", header.GetHeight().String())
	}

	// emitting events in the keeper emits for both begin block and handler client updates
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			eventType,
			sdk.NewAttribute(types.AttributeKeyClientID, clientID),
			sdk.NewAttribute(types.AttributeKeyClientType, clientState.ClientType()),
			sdk.NewAttribute(types.AttributeKeyConsensusHeight, consensusHeight.String()),
			sdk.NewAttribute(types.AttributeKeyHeader, headerStr),
		),
	)

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"ibc", "client", "update"},
			1,
			[]metrics.Label{
				telemetry.NewLabel("client-type", clientState.ClientType()),
				telemetry.NewLabel("client-id", clientID),
				telemetry.NewLabel("update-type", "msg"),
			},
		)
	}()

	return nil
}

// UpgradeClient upgrades the client to a new client state if this new client was committed to
// by the old client at the specified upgrade height
func (k Keeper) UpgradeClient(ctx sdk.Context, clientID string, upgradedClient exported.ClientState, upgradedConsState exported.ConsensusState,
	proofUpgradeClient, proofUpgradeConsState []byte) error {
	clientState, found := k.GetClientState(ctx, clientID)
	if !found {
		return sdkerrors.Wrapf(types.ErrClientNotFound, "cannot update client with ID %s", clientID)
	}

	// prevent upgrade if current client is frozen
	if clientState.IsFrozen() {
		return sdkerrors.Wrapf(types.ErrClientFrozen, "cannot update client with ID %s", clientID)
	}

	updatedClientState, updatedConsState, err := clientState.VerifyUpgradeAndUpdateState(ctx, k.cdc, k.ClientStore(ctx, clientID),
		upgradedClient, upgradedConsState, proofUpgradeClient, proofUpgradeConsState)
	if err != nil {
		return sdkerrors.Wrapf(err, "cannot upgrade client with ID %s", clientID)
	}

	k.SetClientState(ctx, clientID, updatedClientState)
	k.SetClientConsensusState(ctx, clientID, updatedClientState.GetLatestHeight(), updatedConsState)

	k.Logger(ctx).Info("client state upgraded", "client-id", clientID, "height", updatedClientState.GetLatestHeight().String())

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"ibc", "client", "upgrade"},
			1,
			[]metrics.Label{
				telemetry.NewLabel("client-type", updatedClientState.ClientType()),
				telemetry.NewLabel("client-id", clientID),
			},
		)
	}()

	// emitting events in the keeper emits for client upgrades
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeUpgradeClient,
			sdk.NewAttribute(types.AttributeKeyClientID, clientID),
			sdk.NewAttribute(types.AttributeKeyClientType, updatedClientState.ClientType()),
			sdk.NewAttribute(types.AttributeKeyConsensusHeight, updatedClientState.GetLatestHeight().String()),
		),
	)

	return nil
}

// CheckMisbehaviourAndUpdateState checks for client misbehaviour and freezes the
// client if so.
func (k Keeper) CheckMisbehaviourAndUpdateState(ctx sdk.Context, misbehaviour exported.Misbehaviour) error {
	clientState, found := k.GetClientState(ctx, misbehaviour.GetClientID())
	if !found {
		return sdkerrors.Wrapf(types.ErrClientNotFound, "cannot check misbehaviour for client with ID %s", misbehaviour.GetClientID())
	}

	if clientState.IsFrozen() && clientState.GetFrozenHeight().LTE(misbehaviour.GetHeight()) {
		return sdkerrors.Wrapf(types.ErrInvalidMisbehaviour, "client is already frozen at height ≤ misbehaviour height (%s ≤ %s)", clientState.GetFrozenHeight(), misbehaviour.GetHeight())
	}

	clientState, err := clientState.CheckMisbehaviourAndUpdateState(ctx, k.cdc, k.ClientStore(ctx, misbehaviour.GetClientID()), misbehaviour)
	if err != nil {
		return err
	}

	k.SetClientState(ctx, misbehaviour.GetClientID(), clientState)
	k.Logger(ctx).Info("client frozen due to misbehaviour", "client-id", misbehaviour.GetClientID(), "height", misbehaviour.GetHeight().String())

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"ibc", "client", "misbehaviour"},
			1,
			[]metrics.Label{
				telemetry.NewLabel("client-type", misbehaviour.ClientType()),
				telemetry.NewLabel("client-id", misbehaviour.GetClientID()),
			},
		)
	}()

	return nil
}
