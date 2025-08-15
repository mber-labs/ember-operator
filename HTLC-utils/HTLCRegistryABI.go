package HTLC

const HTLCRegistryABI = `[
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "uint256",
				"name": "id",
				"type": "uint256"
			}
		],
		"name": "HTLCCreated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "uint256",
				"name": "id",
				"type": "uint256"
			}
		],
		"name": "Refunded",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "uint256",
				"name": "id",
				"type": "uint256"
			},
			{
				"indexed": false,
				"internalType": "bytes32",
				"name": "secret",
				"type": "bytes32"
			}
		],
		"name": "Withdrawn",
		"type": "event"
	},
	{
		"inputs": [
			{
				"internalType": "address payable",
				"name": "_receiver",
				"type": "address"
			},
			{
				"internalType": "bytes32",
				"name": "_hashLock",
				"type": "bytes32"
			},
			{
				"internalType": "uint256",
				"name": "_timelock",
				"type": "uint256"
			}
		],
		"name": "createHTLC",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "htlcId",
				"type": "uint256"
			}
		],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "_id",
				"type": "uint256"
			}
		],
		"name": "getHTLC",
		"outputs": [
			{
				"components": [
					{
						"internalType": "address payable",
						"name": "sender",
						"type": "address"
					},
					{
						"internalType": "address payable",
						"name": "receiver",
						"type": "address"
					},
					{
						"internalType": "uint256",
						"name": "amount",
						"type": "uint256"
					},
					{
						"internalType": "bytes32",
						"name": "hashLock",
						"type": "bytes32"
					},
					{
						"internalType": "uint256",
						"name": "timelock",
						"type": "uint256"
					},
					{
						"internalType": "bool",
						"name": "withdrawn",
						"type": "bool"
					},
					{
						"internalType": "bool",
						"name": "refunded",
						"type": "bool"
					},
					{
						"internalType": "bytes32",
						"name": "secret",
						"type": "bytes32"
					}
				],
				"internalType": "struct HTLCRegistry.HTLC",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "htlcCount",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"name": "htlcs",
		"outputs": [
			{
				"internalType": "address payable",
				"name": "sender",
				"type": "address"
			},
			{
				"internalType": "address payable",
				"name": "receiver",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"internalType": "bytes32",
				"name": "hashLock",
				"type": "bytes32"
			},
			{
				"internalType": "uint256",
				"name": "timelock",
				"type": "uint256"
			},
			{
				"internalType": "bool",
				"name": "withdrawn",
				"type": "bool"
			},
			{
				"internalType": "bool",
				"name": "refunded",
				"type": "bool"
			},
			{
				"internalType": "bytes32",
				"name": "secret",
				"type": "bytes32"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "_id",
				"type": "uint256"
			}
		],
		"name": "refund",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "_id",
				"type": "uint256"
			},
			{
				"internalType": "bytes32",
				"name": "_secret",
				"type": "bytes32"
			}
		],
		"name": "withdraw",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`