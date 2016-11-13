package wxpay

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// AppTrans is abstact of Transaction handler. With AppTrans, we can get prepay id
type AppTrans struct {
	Config *WxConfig
}

// Initialized the AppTrans with specific config
func NewAppTrans(cfg *WxConfig) (*AppTrans, error) {
	if cfg.AppId == "" ||
		cfg.MchId == "" ||
		cfg.AppKey == "" ||
		cfg.NotifyUrl == "" ||
		cfg.QueryOrderUrl == "" ||
		cfg.PlaceOrderUrl == "" ||
		cfg.TradeType == "" {
		return &AppTrans{Config: cfg}, errors.New("config field canot empty string")
	}

	return &AppTrans{Config: cfg}, nil
}

// Submit the order to weixin pay and return the prepay id if success,
// Prepay id is used for app to start a payment
// If fail, error is not nil, check error for more information
func (this *AppTrans) Submit(params map[string]string) (*PlaceOrderResult, error) {

	odrInXml := this.signedOrderRequestXmlString(params)
	resp, err := doHttpPost(this.Config.PlaceOrderUrl, []byte(odrInXml))
	if err != nil {
		return nil, err
	}

	placeOrderResult, err := ParsePlaceOrderResult(resp)
	if err != nil {
		return nil, err
	}

	if placeOrderResult.ReturnCode != "SUCCESS" {
		return nil, fmt.Errorf("return code:%s, return desc:%s", placeOrderResult.ReturnCode, placeOrderResult.ReturnMsg)
	}

	if placeOrderResult.ResultCode != "SUCCESS" {
		return nil, fmt.Errorf("resutl code:%s, result desc:%s", placeOrderResult.ErrCode, placeOrderResult.ErrCodeDesc)
	}

	//Verify the sign of response
	resultInMap := placeOrderResult.ToMap()
	wantSign := Sign(resultInMap, this.Config.AppKey)
	gotSign := resultInMap["sign"]
	if wantSign != gotSign {
		return nil, fmt.Errorf("sign not match, want:%s, got:%s", wantSign, gotSign)
	}
	return &placeOrderResult, nil
}

func (this *AppTrans) newQueryXml(transId string) string {
	param := make(map[string]string)
	param["appid"] = this.Config.AppId
	param["mch_id"] = this.Config.MchId
	param["transaction_id"] = transId
	param["nonce_str"] = NewNonceString()

	sign := Sign(param, this.Config.AppKey)
	param["sign"] = sign

	return ToXmlString(param)
}

// Query the order from weixin pay server by transaction id of weixin pay
func (this *AppTrans) Query(transId string) (QueryOrderResult, error) {
	queryOrderResult := QueryOrderResult{}

	queryXml := this.newQueryXml(transId)
	// fmt.Println(queryXml)
	resp, err := doHttpPost(this.Config.QueryOrderUrl, []byte(queryXml))
	if err != nil {
		return queryOrderResult, nil
	}

	queryOrderResult, err = ParseQueryOrderResult(resp)
	if err != nil {
		return queryOrderResult, err
	}

	//verity sign of response
	resultInMap := queryOrderResult.ToMap()
	wantSign := Sign(resultInMap, this.Config.AppKey)
	gotSign := resultInMap["sign"]
	if wantSign != gotSign {
		return queryOrderResult, fmt.Errorf("sign not match, want:%s, got:%s", wantSign, gotSign)
	}

	return queryOrderResult, nil
}

// NewPaymentRequest build the payment request structure for app to start a payment.
// Return stuct of PaymentRequest, please refer to http://pay.weixin.qq.com/wiki/doc/api/app.php?chapter=9_12&index=2
func (this *AppTrans) NewPaymentRequest(prepayId string) PaymentRequest {
	noncestr := NewNonceString()
	timestamp := NewTimestampString()

	param := make(map[string]string)
	param["appid"] = this.Config.AppId
	param["partnerid"] = this.Config.MchId
	param["prepayid"] = prepayId
	param["package"] = "Sign=WXPay"
	param["noncestr"] = noncestr
	param["timestamp"] = timestamp

	sign := Sign(param, this.Config.AppKey)

	payRequest := PaymentRequest{
		AppId:     this.Config.AppId,
		PartnerId: this.Config.MchId,
		PrepayId:  prepayId,
		Package:   "Sign=WXPay",
		NonceStr:  noncestr,
		Timestamp: timestamp,
		Sign:      sign,
	}

	return payRequest
}

func (this *AppTrans) newOrderRequest(params map[string]string) map[string]string {
	newParams := make(map[string]string)
	for k, v := range params {
		newParams[k] = v
	}
	newParams["appid"] = this.Config.AppId
	newParams["mch_id"] = this.Config.MchId
	newParams["nonce_str"] = NewNonceString()
	newParams["notify_url"] = this.Config.NotifyUrl
	newParams["device_info"] = "WEB"
	newParams["trade_type"] = this.Config.TradeType

	//test data
	//param["appid"] = "wxd930ea5d5a258f4f"
	//param["mch_id"] = "10000100"
	//param["device_info"] = "1000"
	//param["nonce_str"] = "ibuaiVcKdpRxkhJA"
	//param["body"] = "test"

	return newParams
}

func (this *AppTrans) signedOrderRequestXmlString(params map[string]string) string {
	order := this.newOrderRequest(params)
	sign := Sign(order, this.Config.AppKey)

	order["sign"] = sign

	return ToXmlString(order)
}

// doRequest post the order in xml format with a sign
func doHttpPost(targetUrl string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", targetUrl, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return []byte(""), err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return []byte(""), err
	}

	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte(""), err
	}

	return respData, nil
}
