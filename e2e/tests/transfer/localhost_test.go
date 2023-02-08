package transfer

import (
	"context"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"

	"github.com/cosmos/ibc-go/e2e/testsuite"
	"github.com/cosmos/ibc-go/e2e/testvalues"
	test "github.com/strangelove-ventures/interchaintest/v7/testutil"
)

// TestMsgTransfer_Localhost creates two wallets on a single chain and performs MsgTransfers back and forth
// to ensure ibc functions as expected on localhost. This test is largely the same as TestMsgTransfer_Succeeds_Nonincentivized
// except that chain B is replaced with an additional wallet on chainA.
func (s *TransferTestSuite) TestMsgTransfer_Localhost() {
	t := s.T()
	ctx := context.TODO()

	_, _ = s.SetupChainsRelayerAndChannel(ctx, transferChannelOptions())
	chainA, _ := s.GetChains()

	chainADenom := chainA.Config().Denom

	rlyWallet := s.CreateUserOnChainA(ctx, testvalues.StartingTokenAmount)
	userAWallet := s.CreateUserOnChainA(ctx, testvalues.StartingTokenAmount)
	userBWallet := s.CreateUserOnChainA(ctx, testvalues.StartingTokenAmount)

	var (
		msgChanOpenInitRes channeltypes.MsgChannelOpenInitResponse
		msgChanOpenTryRes  channeltypes.MsgChannelOpenTryResponse
		packet             channeltypes.Packet
	)

	s.Require().NoError(test.WaitForBlocks(ctx, 1, chainA), "failed to wait for blocks")

	t.Run("channel open init localhost", func(t *testing.T) {
		msgChanOpenInit := channeltypes.NewMsgChannelOpenInit(
			transfertypes.PortID, transfertypes.Version,
			channeltypes.UNORDERED, []string{connectiontypes.LocalhostID},
			transfertypes.PortID, rlyWallet.FormattedAddress(),
		)

		s.Require().NoError(msgChanOpenInit.ValidateBasic())

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgChanOpenInit)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)

		s.Require().NoError(testsuite.UnmarshalMsgResponses(txResp, &msgChanOpenInitRes))
	})

	t.Run("channel open try localhost", func(t *testing.T) {
		msgChanOpenTry := channeltypes.NewMsgChannelOpenTry(
			transfertypes.PortID, transfertypes.Version,
			channeltypes.UNORDERED, []string{connectiontypes.LocalhostID},
			transfertypes.PortID, msgChanOpenInitRes.GetChannelId(),
			transfertypes.Version, nil, clienttypes.ZeroHeight(), rlyWallet.FormattedAddress(),
		)

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgChanOpenTry)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)

		s.Require().NoError(testsuite.UnmarshalMsgResponses(txResp, &msgChanOpenTryRes))
	})

	t.Run("channel open ack localhost", func(t *testing.T) {
		msgChanOpenAck := channeltypes.NewMsgChannelOpenAck(
			transfertypes.PortID, msgChanOpenInitRes.GetChannelId(),
			msgChanOpenTryRes.GetChannelId(), transfertypes.Version,
			nil, clienttypes.ZeroHeight(), rlyWallet.FormattedAddress(),
		)

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgChanOpenAck)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)
	})

	t.Run("channel open confirm localhost", func(t *testing.T) {
		msgChanOpenConfirm := channeltypes.NewMsgChannelOpenConfirm(
			transfertypes.PortID, msgChanOpenTryRes.GetChannelId(),
			nil, clienttypes.ZeroHeight(), rlyWallet.FormattedAddress(),
		)

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgChanOpenConfirm)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)
	})

	t.Run("query localhost transfer channel ends", func(t *testing.T) {
		channelEndA, err := s.QueryChannel(ctx, chainA, transfertypes.PortID, msgChanOpenInitRes.GetChannelId())
		s.Require().NoError(err)
		s.Require().NotNil(channelEndA)

		channelEndB, err := s.QueryChannel(ctx, chainA, transfertypes.PortID, msgChanOpenTryRes.GetChannelId())
		s.Require().NoError(err)
		s.Require().NotNil(channelEndB)

		s.Require().Equal(channelEndA.GetConnectionHops(), channelEndB.GetConnectionHops())
	})

	t.Run("send packet - localhost ibc transfer", func(t *testing.T) {
		txResp, err := s.Transfer(ctx, chainA, userAWallet, transfertypes.PortID, "channel-1", testvalues.DefaultTransferAmount(chainADenom), userAWallet.FormattedAddress(), userBWallet.FormattedAddress(), clienttypes.NewHeight(1, 100), 0, "")
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)

		// TODO: revisit parsing packet from events
		// t.Logf("transfer events: %v", txResp.Events)
		// var events sdk.Events
		// for _, evt := range txResp.Events {
		// 	var attributes []sdk.Attribute
		// 	for _, attr := range evt.GetAttributes() {
		// 		attributes = append(attributes, sdk.NewAttribute(attr.Key, attr.Value))
		// 	}

		// 	events.AppendEvent(sdk.NewEvent(evt.GetType(), attributes...))
		// }

		// packet, err = ibctesting.ParsePacketFromEvents(events)
		// s.Require().NoError(err)
		// s.Require().NotNil(packet)
	})

	t.Run("tokens are escrowed", func(t *testing.T) {
		actualBalance, err := s.GetChainANativeBalance(ctx, userAWallet)
		s.Require().NoError(err)

		expected := testvalues.StartingTokenAmount - testvalues.IBCTransferAmount
		s.Require().Equal(expected, actualBalance)
	})

	t.Run("recv packet - localhost ibc transfer", func(t *testing.T) {
		// TODO: currently building the packet manually, should be possible to parse from events
		packet = channeltypes.NewPacket(transfertypes.NewFungibleTokenPacketData(chainADenom, "10000", userAWallet.FormattedAddress(), userBWallet.FormattedAddress(), "").GetBytes(), 1, "transfer", "channel-1", "transfer", "channel-2", clienttypes.NewHeight(1, 100), 0)
		msgRecvPacket := channeltypes.NewMsgRecvPacket(packet, nil, clienttypes.ZeroHeight(), rlyWallet.FormattedAddress())

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgRecvPacket)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)
	})

	t.Run("acknowledge packet - localhost ibc transfer", func(t *testing.T) {
		// TODO: currently building the packet manually, should be possible to parse from events
		packet = channeltypes.NewPacket(transfertypes.NewFungibleTokenPacketData(chainADenom, "10000", userAWallet.FormattedAddress(), userBWallet.FormattedAddress(), "").GetBytes(), 1, "transfer", "channel-1", "transfer", "channel-2", clienttypes.NewHeight(1, 100), 0)
		msgAcknowledgement := channeltypes.NewMsgAcknowledgement(
			packet, channeltypes.NewResultAcknowledgement([]byte{byte(1)}).Acknowledgement(),
			nil, clienttypes.ZeroHeight(), rlyWallet.FormattedAddress(),
		)

		txResp, err := s.BroadcastMessages(ctx, chainA, rlyWallet, msgAcknowledgement)
		s.Require().NoError(err)
		s.AssertValidTxResponse(txResp)
	})

	t.Run("packets are relayed", func(t *testing.T) {
		s.AssertPacketRelayed(ctx, chainA, transfertypes.PortID, "channel-1", 1)

		ibcToken := testsuite.GetIBCToken(chainADenom, transfertypes.PortID, "channel-2")
		actualBalance, err := chainA.GetBalance(ctx, userBWallet.FormattedAddress(), ibcToken.IBCDenom())
		s.Require().NoError(err)

		expected := testvalues.IBCTransferAmount
		s.Require().Equal(expected, actualBalance)
	})
}
