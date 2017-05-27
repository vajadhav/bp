package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/crypto/primitives"
)

var logger = shim.NewLogger("UFAChainCode")

//ALL_ELEMENENTS Key to refer the master list of UFA
const ALL_ELEMENENTS = "ALL_RECS"

//UFA_TRXN_PREFIX Key prefix for UFA transaction history
const UFA_TRXN_PREFIX = "UFA_TRXN_HISTORY_"

//UFA_INVOICE_PREFIX Key prefix for identifying Invoices assciated with a ufa
const UFA_INVOICE_PREFIX = "UFA_INVOICE_PREFIX_"

//UFAChainCode Chaincode default interface
type UFAChainCode struct {
}

//Validate Invoice
func (t *UFAChainCode) validateInvoiceDetails(stub shim.ChaincodeStubInterface, args []string) string {

	logger.Info("validateInvoice called")
	var validationMessage bytes.Buffer
	ext := UFAChainCode{}
	//who := args[0]
	payload := args[1]
	//I am assuming the payload will be an array of Invoices
	//Once for cusotmer and another for vendor
	//Checking only one would be sufficient from the amount perspective
	var invoiceList []map[string]string
	json.Unmarshal([]byte(payload), &invoiceList)
	if len(invoiceList) < 2 {
		validationMessage.WriteString("\nInvoice is missing for Customer or Vendor")
	} else {
		//Get the UFA number
		ufanumber := invoiceList[0]["ufanumber"]
		var ufaDetails map[string]string
		//who :=args[1] //Role
		//Get the ufaDetails
		recBytes, err := stub.GetState(ufanumber)
		if err != nil {
			validationMessage.WriteString("\nInvalid UFA provided")
		} else {
			json.Unmarshal(recBytes, &ufaDetails)
			tolerence := ext.validateNumber(ufaDetails["chargTolrence"])
			netCharge := ext.validateNumber(ufaDetails["netCharge"])

			raisedInvTotal := ext.validateNumber(ufaDetails["raisedInvTotal"])
			//Calculate the max charge
			maxCharge := netCharge + netCharge*tolerence/100.0
			//We are assumming 2 invoices have the same amount in it
			invAmt1 := ext.validateNumber(invoiceList[0]["invoiceAmt"])
			invAmt2 := ext.validateNumber(invoiceList[1]["invoiceAmt"])
			billingPeriod := invoiceList[0]["billingPeriod"]
			if ext.checkInvoicesRaised(stub, ufanumber, billingPeriod) {
				validationMessage.WriteString("\nInvoice all already raised for " + billingPeriod)
			} else if invAmt1 != invAmt2 {
				validationMessage.WriteString("\nCustomer and Vendor Invoice Amounts are not same")
			} else if maxCharge < (invAmt1 + raisedInvTotal) {
				validationMessage.WriteString("\nTotal invoice amount exceded")
			}
		} // Invalid UFA number
	} // End of length of invoics
	finalMessage := validationMessage.String()
	logger.Info("validateInvoice Validation message generated :" + finalMessage)
	return finalMessage
}

//Checking if invoice is already raised or not
func (t *UFAChainCode) checkInvoicesRaised(stub shim.ChaincodeStubInterface, ufaNumber string, billingPeriod string) bool {
	ext := UFAChainCode{}
	var isAvailable = false
	logger.Info("checkInvoicesRaised started for :" + ufaNumber + " : Billing month " + billingPeriod)
	allInvoices := ext.getInvoicesForUFA(stub, ufaNumber)
	if len(allInvoices) > 0 {
		for _, invoiceDetails := range allInvoices {
			logger.Info("checkInvoicesRaised checking for invoice number :" + invoiceDetails["invoiceNumber"])
			if invoiceDetails["billingPeriod"] == billingPeriod {
				isAvailable = true
				break
			}
		}
	}
	return isAvailable
}

//Returns all the invoices raised for an UFA
func (t *UFAChainCode) getInvoicesForUFA(stub shim.ChaincodeStubInterface, ufanumber string) []map[string]string {
	logger.Info("getInvoicesForUFA called")
	ext := UFAChainCode{}
	var outputRecords []map[string]string
	outputRecords = make([]map[string]string, 0)

	recordsList, err := ext.getAllInvloiceList(stub, ufanumber)
	if err == nil {
		for _, invoiceNumber := range recordsList {
			logger.Info("getInvoicesForUFA: Processing record " + ufanumber)
			recBytes, _ := stub.GetState(invoiceNumber)
			var record map[string]string
			json.Unmarshal(recBytes, &record)
			outputRecords = append(outputRecords, record)
		}

	}

	logger.Info("Returning records from getInvoicesForUFA ")
	return outputRecords
}

func (t *UFAChainCode) getAllInvloiceList(stub shim.ChaincodeStubInterface, ufanumber string) ([]string, error) {
	var recordList []string
	recBytes, _ := stub.GetState(UFA_INVOICE_PREFIX + ufanumber)

	err := json.Unmarshal(recBytes, &recordList)
	if err != nil {
		return nil, errors.New("Failed to unmarshal getAllInvloiceList ")
	}

	return recordList, nil
}

//Append a new UFA numbetr to the master list
func (t *UFAChainCode) updateMasterRecords(stub shim.ChaincodeStubInterface, ufaNumber string) error {
	var recordList []string
	recBytes, _ := stub.GetState(ALL_ELEMENENTS)

	err := json.Unmarshal(recBytes, &recordList)
	if err != nil {
		return errors.New("Failed to unmarshal updateMasterReords ")
	}
	recordList = append(recordList, ufaNumber)
	bytesToStore, _ := json.Marshal(recordList)
	logger.Info("After addition" + string(bytesToStore))
	stub.PutState(ALL_ELEMENENTS, bytesToStore)
	return nil
}

//Append to UFA transaction history
func (t *UFAChainCode) appendUFATransactionHistory(stub shim.ChaincodeStubInterface, ufanumber string, payload string) error {
	var recordList []string

	logger.Info("Appending to transaction history " + ufanumber)
	recBytes, _ := stub.GetState(UFA_TRXN_PREFIX + ufanumber)

	if recBytes == nil {
		logger.Info("Updating the transaction history for the first time")
		recordList = make([]string, 0)
	} else {
		err := json.Unmarshal(recBytes, &recordList)
		if err != nil {
			return errors.New("Failed to unmarshal appendUFATransactionHistory ")
		}
	}
	recordList = append(recordList, payload)
	bytesToStore, _ := json.Marshal(recordList)
	logger.Info("After updating the transaction history" + string(bytesToStore))
	stub.PutState(UFA_TRXN_PREFIX+ufanumber, bytesToStore)
	logger.Info("Appending to transaction history " + ufanumber + " Done!!")
	return nil
}

//Returns all the UFA Numbers stored
func (t *UFAChainCode) getAllRecordsList(stub shim.ChaincodeStubInterface) ([]string, error) {
	var recordList []string
	recBytes, _ := stub.GetState(ALL_ELEMENENTS)

	err := json.Unmarshal(recBytes, &recordList)
	if err != nil {
		return nil, errors.New("Failed to unmarshal getAllRecordsList ")
	}

	return recordList, nil
}

// Creating a new Upfront agreement
func (t *UFAChainCode) createUFA(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	logger.Info("createUFA called")

	ufanumber := args[0]
	who := args[1]
	payload := args[2]
	//If there is no error messages then create the UFA
	valMsg := validateNewUFA(who, payload)
	if valMsg == "" {
		stub.PutState(ufanumber, []byte(payload))
		ext := UFAChainCode{}
		ext.updateMasterRecords(stub, ufanumber)
		ext.appendUFATransactionHistory(stub, ufanumber, payload)
		logger.Info("Created the UFA after successful validation : " + payload)
	} else {
		return nil, errors.New("Validation failure: " + valMsg)
	}
	return nil, nil
}

//Validate a new UFA
func validateNewUFA(who string, payload string) string {
	ext := UFAChainCode{}
	//As of now I am checking if who is of proper role
	var validationMessage bytes.Buffer
	var ufaDetails map[string]string

	logger.Info("validateNewUFA")
	if who == "SELLER" || who == "BUYER" {
		json.Unmarshal([]byte(payload), &ufaDetails)
		//Now check individual fields
		netChargeStr := ufaDetails["netCharge"]
		tolerenceStr := ufaDetails["chargTolrence"]
		netCharge := ext.validateNumber(netChargeStr)
		if netCharge <= 0.0 {
			validationMessage.WriteString("\nInvalid net charge")
		}
		tolerence := ext.validateNumber(tolerenceStr)
		if tolerence <= 0.0 || tolerence > 10.0 {
			validationMessage.WriteString("\nTolerence is out of range. Should be between 0 and 10")
		}

	} else {
		validationMessage.WriteString("\nUser is not authorized to create a UFA")
	}
	logger.Info("Validation messagge " + validationMessage.String())
	return validationMessage.String()
}

//Validate a input string as number or not
func (t *UFAChainCode) validateNumber(str string) float64 {
	if netCharge, err := strconv.ParseFloat(str, 64); err == nil {
		return netCharge
	}
	return float64(-1.0)
}

//Update the existing record with the mofied key value pair
func (t *UFAChainCode) updateRecord(existingRecord map[string]string, fieldsToUpdate map[string]string) (string, error) {
	for key, value := range fieldsToUpdate {

		existingRecord[key] = value
	}
	outputMapBytes, _ := json.Marshal(existingRecord)
	logger.Info("updateRecord: Final json after update " + string(outputMapBytes))
	return string(outputMapBytes), nil
}

// Update and existing UFA record
func (t *UFAChainCode) updateUFA(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var existingRecMap map[string]string
	var updatedFields map[string]string

	logger.Info("updateUFA called ")
	ext := UFAChainCode{}

	ufanumber := args[0]
	//TODO: Update the validation here
	//who := args[1]
	payload := args[2]
	logger.Info("updateUFA payload passed " + payload)

	//who :=args[2]
	recBytes, _ := stub.GetState(ufanumber)

	json.Unmarshal(recBytes, &existingRecMap)
	json.Unmarshal([]byte(payload), &updatedFields)
	updatedReord, _ := ext.updateRecord(existingRecMap, updatedFields)
	//Store the records
	stub.PutState(ufanumber, []byte(updatedReord))
	ext.appendUFATransactionHistory(stub, ufanumber, payload)
	return nil, nil
}

//Returns all the UFAs created so far
func (t *UFAChainCode) getAllUFA(stub shim.ChaincodeStubInterface, who string) ([]byte, error) {
	logger.Info("getAllUFA called")
	ext := UFAChainCode{}
	recordsList, err := ext.getAllRecordsList(stub)
	if err != nil {
		return nil, errors.New("Unable to get all the records ")
	}
	var outputRecords []map[string]string
	outputRecords = make([]map[string]string, 0)
	for _, ufanumber := range recordsList {
		logger.Info("getAllUFA: Processing record " + ufanumber)
		recBytes, _ := stub.GetState(ufanumber)
		var record map[string]string
		json.Unmarshal(recBytes, &record)
		outputRecords = append(outputRecords, record)
	}
	outputBytes, _ := json.Marshal(outputRecords)
	logger.Info("Returning records from getAllUFA " + string(outputBytes))
	return outputBytes, nil
}

//Get a single ufa
func (t *UFAChainCode) getUFADetails(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	logger.Info("getUFADetails called with UFA number: " + args[0])

	var outputRecord map[string]string
	ufanumber := args[0] //UFA ufanum
	//who :=args[1] //Role
	recBytes, _ := stub.GetState(ufanumber)
	json.Unmarshal(recBytes, &outputRecord)
	outputBytes, _ := json.Marshal(outputRecord)
	logger.Info("Returning records from getUFADetails " + string(outputBytes))
	return outputBytes, nil
}

// Init initializes the smart contracts
func (t *UFAChainCode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	logger.Info("Init called")
	//Place an empty arry
	stub.PutState(ALL_ELEMENENTS, []byte("[]"))
	return nil, nil
}
func (t *UFAChainCode) probe() []byte {
	ts := time.Now().Format(time.UnixDate)
	output := "{\"status\":\"Success\",\"ts\" : \"" + ts + "\" }"
	return []byte(output)
}

//Validate the new UFA
func (t *UFAChainCode) validateNewUFAData(args []string) []byte {
	var output string
	msg := validateNewUFA(args[0], args[1])

	if msg == "" {
		output = "{\"validation\":\"Success\",\"msg\" : \"\" }"
	} else {
		output = "{\"validation\":\"Failure\",\"msg\" : \"" + msg + "\" }"
	}
	return []byte(output)
}

// Invoke entry point
func (t *UFAChainCode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	logger.Info("Invoke called")
	ext := UFAChainCode{}

	if function == "createUFA" {
		ext.createUFA(stub, args)
	} else if function == "updateUFA" {
		ext.updateUFA(stub, args)
	}

	return nil, nil
}

// Query the rcords form the  smart contracts
func (t *UFAChainCode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	logger.Info("Query called")
	ext := UFAChainCode{}
	if function == "getAllUFA" {
		return ext.getAllUFA(stub, args[0])
	} else if function == "getUFADetails" {
		return ext.getUFADetails(stub, args)
	} else if function == "probe" {
		return ext.probe(), nil
	} else if function == "validateNewUFA" {
		return ext.validateNewUFAData(args), nil
	} else if function == "validateInvoiceDetails" {
		return []byte(ext.validateInvoiceDetails(stub, args)), nil
	}

	return nil, nil
}

//Main method
func main() {
	logger.SetLevel(shim.LogInfo)
	primitives.SetSecurityLevel("SHA3", 256)
	err := shim.Start(new(UFAChainCode))
	if err != nil {
		fmt.Printf("Error starting UFAChainCode: %s", err)
	}
}
