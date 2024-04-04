const protobuf = require("protobufjs");
const axios = require("axios");

protobuf.load("../internal/bft/types/transaction.proto", async function (err, root) {
    if (err) {
        throw err;
    }

    // Resolve the 'Transaction' message type
    const Transaction = root.lookupType("Transaction");
    
    // Create a new message instance for Node Registration Transaction
    const nodeRegistrationMessage = Transaction.create({
        owner: "exampleOwner",
        signature: "exampleSignature",
        type: root.lookupEnum("TransactionType").values.NodeRegistrationTransaction, // Use enum value
        nodeRegistrationData: { // Updated field name
            nodeAddress: "sFGf1bQ884mUrTpQNQ4VXpPlS7uyMqz/ohRdH35kR8c=", // Updated field name
            nodeAttestation: "exampleNodeAttestation", // Updated field name
            nodeSignature: "exampleNodeSignature" // Updated field name
        }
    });

    // Verify the message
    const errMsg = Transaction.verify(nodeRegistrationMessage);
    if (errMsg) {
        throw new Error(errMsg);
    }
    console.log(nodeRegistrationMessage)
    // Encode the message to a Uint8Array (binary format)
    const buffer = Transaction.encode(nodeRegistrationMessage).finish();
    const hexString = Buffer.from(buffer).toString('hex');

    // Log the binary payload and hex string
    console.log(buffer);
    console.log(hexString);

    // Make a POST request to broadcast the transaction
    const url = "http://localhost:26657/broadcast_tx_commit?tx=" + hexString;
    console.log(url)
    try {
        const response = await axios.post(url);
        console.log("Transaction response:", response.data);
    } catch (error) {
        console.error("Transaction error:", error.response.data);
    }
});
