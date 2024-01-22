package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
)

func (suite *KeeperTestSuite) TestMsgConnectionOpenInitEvents() {
	suite.SetupTest()
	path := ibctesting.NewPath(suite.chainA, suite.chainB)
	suite.coordinator.SetupClients(path)

	msg := types.NewMsgConnectionOpenInit(
		path.EndpointA.ClientID,
		path.EndpointA.Counterparty.ClientID,
		path.EndpointA.Counterparty.Chain.GetPrefix(), ibctesting.DefaultOpenInitVersion, path.EndpointA.ConnectionConfig.DelayPeriod,
		path.EndpointA.Chain.SenderAccount.GetAddress().String(),
	)

	res, err := suite.chainA.SendMsgs(msg)
	suite.Require().NoError(err)
	suite.Require().NotNil(res)

	events := res.Events
	expectedEvents := sdk.Events{
		sdk.NewEvent(
			types.EventTypeConnectionOpenInit,
			sdk.NewAttribute(types.AttributeKeyConnectionID, ibctesting.FirstConnectionID),
			sdk.NewAttribute(types.AttributeKeyClientID, path.EndpointA.ClientID),
			sdk.NewAttribute(types.AttributeKeyCounterpartyClientID, path.EndpointB.ClientID),
		),
	}.ToABCIEvents()

	var indexSet map[string]struct{}
	expectedEvents = sdk.MarkEventsToIndex(expectedEvents, indexSet)
	ibctesting.AssertEvents(&suite.Suite, expectedEvents, events)
}
