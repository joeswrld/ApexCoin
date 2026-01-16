import hashlib
import json
from time import time
from urllib.parse import urlparse
from uuid import uuid4
import base64
from Crypto.PublicKey import RSA
from Crypto.Signature import pkcs1_15
from Crypto.Hash import SHA256

import requests
from flask import Flask, jsonify, request
from flask_cors import CORS

# ApexCoin Configuration
BLOCKCHAIN_NAME = "ApexCoin"
COIN_SYMBOL = "APEX"
VERSION = "1.0.0"


class Wallet:
    """Secure wallet with RSA cryptographic signing"""
    
    def __init__(self):
        self.private_key = RSA.generate(2048)
        self.public_key = self.private_key.publickey()
        
    def get_address(self):
        """Generate unique wallet address from public key"""
        public_key_bytes = self.public_key.export_key()
        address = hashlib.sha256(public_key_bytes).hexdigest()
        return address
    
    def sign_transaction(self, transaction):
        """Sign transaction with private key for authenticity"""
        transaction_string = json.dumps(transaction, sort_keys=True).encode()
        hash_obj = SHA256.new(transaction_string)
        signature = pkcs1_15.new(self.private_key).sign(hash_obj)
        return base64.b64encode(signature).decode()
    
    def verify_signature(self, transaction, signature, public_key):
        """Verify transaction signature"""
        try:
            transaction_string = json.dumps(transaction, sort_keys=True).encode()
            hash_obj = SHA256.new(transaction_string)
            signature_bytes = base64.b64decode(signature)
            pkcs1_15.new(public_key).verify(hash_obj, signature_bytes)
            return True
        except:
            return False
    
    def export_keys(self):
        """Export keys for backup"""
        return {
            'private_key': self.private_key.export_key().decode(),
            'public_key': self.public_key.export_key().decode(),
            'address': self.get_address()
        }
    
    @staticmethod
    def import_keys(private_key_string):
        """Import wallet from private key"""
        wallet = Wallet()
        wallet.private_key = RSA.import_key(private_key_string)
        wallet.public_key = wallet.private_key.publickey()
        return wallet


class ApexCoinBlockchain:
    """ApexCoin Blockchain with advanced features"""
    
    def __init__(self):
        self.current_transactions = []
        self.chain = []
        self.nodes = set()
        self.wallets = {}
        
        # Mining configuration
        self.mining_reward = 50  # Initial mining reward
        self.base_difficulty = 4  # Number of leading zeros required
        self.difficulty_adjustment_interval = 10  # Adjust every N blocks
        self.target_block_time = 60  # Target 60 seconds per block
        self.halving_interval = 210000  # Halve rewards every 210k blocks (like Bitcoin)
        self.max_supply = 21000000  # Maximum 21 million coins
        
        # Transaction fees
        self.min_transaction_fee = 0.001
        self.max_transaction_size = 1000  # Max transactions per block
        
        # Mining statistics
        self.mining_stats = {
            'total_blocks_mined': 0,
            'total_rewards_distributed': 0,
            'total_supply': 0,
            'miners': {}
        }
        
        # Mining pool
        self.mining_pool = {
            'members': {},
            'total_shares': 0
        }

        # Create genesis block
        self.new_block(previous_hash='0', proof=100, miner_address='genesis')

    def register_node(self, address):
        """Add a new node to the network"""
        parsed_url = urlparse(address)
        if parsed_url.netloc:
            self.nodes.add(parsed_url.netloc)
        elif parsed_url.path:
            self.nodes.add(parsed_url.path)
        else:
            raise ValueError('Invalid URL')

    def valid_chain(self, chain):
        """Validate blockchain integrity"""
        last_block = chain[0]
        current_index = 1

        while current_index < len(chain):
            block = chain[current_index]
            
            # Verify hash linkage
            last_block_hash = self.hash(last_block)
            if block['previous_hash'] != last_block_hash:
                return False

            # Verify proof of work
            if not self.valid_proof(last_block['proof'], block['proof'], 
                                   last_block_hash, block.get('difficulty', self.base_difficulty)):
                return False

            last_block = block
            current_index += 1

        return True

    def resolve_conflicts(self):
        """Consensus algorithm - longest valid chain wins"""
        neighbours = self.nodes
        new_chain = None
        max_length = len(self.chain)

        for node in neighbours:
            try:
                response = requests.get(f'http://{node}/chain', timeout=5)
                if response.status_code == 200:
                    length = response.json()['length']
                    chain = response.json()['chain']

                    if length > max_length and self.valid_chain(chain):
                        max_length = length
                        new_chain = chain
            except:
                continue

        if new_chain:
            self.chain = new_chain
            self.recalculate_balances()
            return True

        return False

    def recalculate_balances(self):
        """Rebuild all wallet balances from blockchain"""
        self.wallets = {}
        for block in self.chain:
            for transaction in block['transactions']:
                sender = transaction['sender']
                recipient = transaction['recipient']
                amount = transaction['amount']
                
                if sender != "0":
                    self.wallets[sender] = self.wallets.get(sender, 0) - amount
                
                self.wallets[recipient] = self.wallets.get(recipient, 0) + amount

    def get_current_difficulty(self):
        """Dynamic difficulty adjustment based on block times"""
        chain_length = len(self.chain)
        
        if chain_length < self.difficulty_adjustment_interval:
            return self.base_difficulty
        
        if chain_length % self.difficulty_adjustment_interval == 0:
            recent_blocks = self.chain[-self.difficulty_adjustment_interval:]
            time_taken = recent_blocks[-1]['timestamp'] - recent_blocks[0]['timestamp']
            avg_block_time = time_taken / self.difficulty_adjustment_interval
            
            # Increase difficulty if blocks are too fast
            if avg_block_time < self.target_block_time * 0.5:
                return min(self.base_difficulty + 1, 6)
            # Decrease if too slow
            elif avg_block_time > self.target_block_time * 2:
                return max(self.base_difficulty - 1, 3)
        
        return self.base_difficulty

    def get_current_reward(self):
        """Calculate mining reward with halving"""
        halvings = len(self.chain) // self.halving_interval
        reward = self.mining_reward / (2 ** halvings)
        
        # Check max supply
        if self.mining_stats['total_supply'] + reward > self.max_supply:
            return max(0, self.max_supply - self.mining_stats['total_supply'])
        
        return max(reward, 0.00000001)  # Minimum reward

    def new_block(self, proof, previous_hash, miner_address='genesis'):
        """Create a new block and add to chain"""
        block = {
            'index': len(self.chain) + 1,
            'timestamp': time(),
            'transactions': self.current_transactions,
            'proof': proof,
            'previous_hash': previous_hash or self.hash(self.chain[-1]),
            'miner': miner_address,
            'difficulty': self.get_current_difficulty() if self.chain else self.base_difficulty,
            'reward': self.get_current_reward() if self.chain else 0
        }

        # Update balances
        for transaction in self.current_transactions:
            sender = transaction['sender']
            recipient = transaction['recipient']
            amount = transaction['amount']
            
            if sender != "0":
                self.wallets[sender] = self.wallets.get(sender, 0) - amount
            
            self.wallets[recipient] = self.wallets.get(recipient, 0) + amount

        # Update statistics
        if miner_address != 'genesis':
            self.mining_stats['total_blocks_mined'] += 1
            self.mining_stats['total_rewards_distributed'] += block['reward']
            self.mining_stats['total_supply'] += block['reward']
            
            if miner_address not in self.mining_stats['miners']:
                self.mining_stats['miners'][miner_address] = {
                    'blocks_mined': 0,
                    'total_rewards': 0,
                    'last_block_time': None
                }
            
            self.mining_stats['miners'][miner_address]['blocks_mined'] += 1
            self.mining_stats['miners'][miner_address]['total_rewards'] += block['reward']
            self.mining_stats['miners'][miner_address]['last_block_time'] = block['timestamp']

        self.current_transactions = []
        self.chain.append(block)
        return block

    def new_transaction(self, sender, recipient, amount, signature=None, fee=0):
        """Create a new transaction with validation"""
        # Validate transaction
        if sender != "0":
            total_cost = amount + fee
            if self.get_balance(sender) < total_cost:
                raise ValueError(f"Insufficient balance. Available: {self.get_balance(sender)}, Required: {total_cost}")
            
            if fee < self.min_transaction_fee:
                raise ValueError(f"Transaction fee must be at least {self.min_transaction_fee}")
            
            if not signature:
                raise ValueError("Transaction must be signed")
        
        transaction = {
            'sender': sender,
            'recipient': recipient,
            'amount': amount,
            'fee': fee,
            'signature': signature,
            'timestamp': time()
        }
        
        self.current_transactions.append(transaction)
        return self.last_block['index'] + 1

    def get_balance(self, address):
        """Get wallet balance"""
        return self.wallets.get(address, 0)

    def get_transaction_fees(self):
        """Calculate total transaction fees"""
        return sum(tx.get('fee', 0) for tx in self.current_transactions)

    @property
    def last_block(self):
        return self.chain[-1]

    @staticmethod
    def hash(block):
        """Create SHA-256 hash of block"""
        block_string = json.dumps(block, sort_keys=True).encode()
        return hashlib.sha256(block_string).hexdigest()

    def proof_of_work(self, last_block, difficulty=None):
        """
        Proof of Work algorithm with metrics
        Returns: (proof, attempts, time_taken)
        """
        if difficulty is None:
            difficulty = self.get_current_difficulty()
            
        last_proof = last_block['proof']
        last_hash = self.hash(last_block)

        proof = 0
        attempts = 0
        start_time = time()
        
        while self.valid_proof(last_proof, proof, last_hash, difficulty) is False:
            proof += 1
            attempts += 1

        time_taken = time() - start_time
        return proof, attempts, time_taken

    @staticmethod
    def valid_proof(last_proof, proof, last_hash, difficulty=4):
        """Validate proof of work"""
        guess = f'{last_proof}{proof}{last_hash}'.encode()
        guess_hash = hashlib.sha256(guess).hexdigest()
        return guess_hash[:difficulty] == "0" * difficulty

    # Mining Pool Methods
    def join_mining_pool(self, address):
        """Join the mining pool"""
        if address not in self.mining_pool['members']:
            self.mining_pool['members'][address] = 0
            return True
        return False

    def leave_mining_pool(self, address):
        """Leave the mining pool"""
        if address in self.mining_pool['members']:
            del self.mining_pool['members'][address]
            return True
        return False


# Flask Application
app = Flask(__name__)
CORS(app)

node_identifier = str(uuid4()).replace('-', '')
blockchain = ApexCoinBlockchain()
created_wallets = {}


@app.route('/info', methods=['GET'])
def blockchain_info():
    """Get blockchain information"""
    response = {
        'name': BLOCKCHAIN_NAME,
        'symbol': COIN_SYMBOL,
        'version': VERSION,
        'total_supply': blockchain.mining_stats['total_supply'],
        'max_supply': blockchain.max_supply,
        'blocks': len(blockchain.chain),
        'difficulty': blockchain.get_current_difficulty(),
        'reward': blockchain.get_current_reward(),
        'nodes': len(blockchain.nodes)
    }
    return jsonify(response), 200


@app.route('/wallet/new', methods=['GET'])
def new_wallet():
    """Create a new wallet"""
    wallet = Wallet()
    wallet_data = wallet.export_keys()
    address = wallet.get_address()
    created_wallets[address] = wallet
    
    response = {
        'message': f'{BLOCKCHAIN_NAME} wallet created',
        'address': address,
        'private_key': wallet_data['private_key'],
        'public_key': wallet_data['public_key'],
        'balance': blockchain.get_balance(address),
        'warning': '⚠️ SAVE YOUR PRIVATE KEY! You need it to sign transactions.'
    }
    return jsonify(response), 201


@app.route('/wallet/balance/<address>', methods=['GET'])
def get_balance(address):
    """Get wallet balance"""
    response = {
        'address': address,
        'balance': blockchain.get_balance(address),
        'symbol': COIN_SYMBOL
    }
    return jsonify(response), 200


@app.route('/mine', methods=['GET', 'POST'])
def mine():
    """Mine a new block"""
    if request.method == 'POST':
        values = request.get_json()
        miner_address = values.get('miner_address', node_identifier)
    else:
        miner_address = request.args.get('miner_address', node_identifier)
    
    last_block = blockchain.last_block
    difficulty = blockchain.get_current_difficulty()
    reward = blockchain.get_current_reward()
    
    # Calculate proof of work
    proof, attempts, time_taken = blockchain.proof_of_work(last_block, difficulty)
    
    # Calculate fees
    tx_fees = blockchain.get_transaction_fees()
    total_reward = reward + tx_fees

    # Add mining reward
    blockchain.new_transaction(
        sender="0",
        recipient=miner_address,
        amount=total_reward,
    )

    # Create block
    previous_hash = blockchain.hash(last_block)
    block = blockchain.new_block(proof, previous_hash, miner_address)

    response = {
        'message': f"⛏️ New {BLOCKCHAIN_NAME} Block Mined!",
        'block': {
            'index': block['index'],
            'timestamp': block['timestamp'],
            'transactions': len(block['transactions']),
            'proof': block['proof'],
            'previous_hash': block['previous_hash'],
        },
        'mining_info': {
            'miner_address': miner_address,
            'difficulty': difficulty,
            'attempts': attempts,
            'time_taken': round(time_taken, 2),
            'hashrate': round(attempts / time_taken if time_taken > 0 else 0, 2),
            'base_reward': reward,
            'transaction_fees': tx_fees,
            'total_reward': total_reward,
        },
        'miner_reward': total_reward,
        'new_balance': blockchain.get_balance(miner_address)
    }
    return jsonify(response), 200


@app.route('/mine/stats', methods=['GET'])
def mining_stats():
    """Get mining statistics"""
    response = {
        'network_stats': {
            'total_blocks': len(blockchain.chain),
            'total_supply': blockchain.mining_stats['total_supply'],
            'max_supply': blockchain.max_supply,
            'percent_mined': (blockchain.mining_stats['total_supply'] / blockchain.max_supply) * 100,
            'total_rewards_distributed': blockchain.mining_stats['total_rewards_distributed'],
            'current_difficulty': blockchain.get_current_difficulty(),
            'current_reward': blockchain.get_current_reward(),
            'pending_transactions': len(blockchain.current_transactions),
        },
        'top_miners': sorted(
            [
                {
                    'address': addr,
                    'blocks_mined': stats['blocks_mined'],
                    'total_rewards': stats['total_rewards'],
                    'avg_reward': stats['total_rewards'] / stats['blocks_mined'] if stats['blocks_mined'] > 0 else 0
                }
                for addr, stats in blockchain.mining_stats['miners'].items()
            ],
            key=lambda x: x['blocks_mined'],
            reverse=True
        )[:10]
    }
    return jsonify(response), 200


@app.route('/transactions/send', methods=['POST'])
def send_transaction():
    """Send coins with signature verification"""
    values = request.get_json()
    
    required = ['sender_address', 'recipient_address', 'amount', 'private_key']
    if not all(k in values for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    
    try:
        wallet = Wallet.import_keys(values['private_key'])
        
        if wallet.get_address() != values['sender_address']:
            return jsonify({'error': 'Private key does not match sender address'}), 400
        
        fee = values.get('fee', blockchain.min_transaction_fee)
        
        transaction_data = {
            'sender': values['sender_address'],
            'recipient': values['recipient_address'],
            'amount': values['amount']
        }
        
        signature = wallet.sign_transaction(transaction_data)
        
        index = blockchain.new_transaction(
            values['sender_address'],
            values['recipient_address'],
            values['amount'],
            signature,
            fee
        )
        
        response = {
            'message': f'Transaction will be added to Block {index}',
            'transaction': {
                'sender': values['sender_address'],
                'recipient': values['recipient_address'],
                'amount': values['amount'],
                'fee': fee,
                'total': values['amount'] + fee
            },
            'sender_balance_after_confirmation': blockchain.get_balance(values['sender_address']) - values['amount'] - fee
        }
        return jsonify(response), 201
        
    except ValueError as e:
        return jsonify({'error': str(e)}), 400


@app.route('/faucet', methods=['POST'])
def faucet():
    """Get free test coins"""
    values = request.get_json()
    
    if 'address' not in values:
        return jsonify({'error': 'Address required'}), 400
    
    faucet_amount = 100
    
    blockchain.new_transaction(
        sender="0",
        recipient=values['address'],
        amount=faucet_amount
    )
    
    last_block = blockchain.last_block
    proof, attempts, time_taken = blockchain.proof_of_work(last_block)
    previous_hash = blockchain.hash(last_block)
    block = blockchain.new_block(proof, previous_hash, 'faucet')
    
    response = {
        'message': f'{faucet_amount} {COIN_SYMBOL} sent to {values["address"]}',
        'new_balance': blockchain.get_balance(values['address']),
        'block_index': block['index']
    }
    return jsonify(response), 200


@app.route('/chain', methods=['GET'])
def full_chain():
    """Get the full blockchain"""
    response = {
        'chain': blockchain.chain,
        'length': len(blockchain.chain),
    }
    return jsonify(response), 200


@app.route('/nodes/register', methods=['POST'])
def register_nodes():
    """Register new nodes"""
    values = request.get_json()
    nodes = values.get('nodes')
    
    if nodes is None:
        return "Error: Please supply a valid list of nodes", 400

    for node in nodes:
        blockchain.register_node(node)

    response = {
        'message': 'New nodes have been added',
        'total_nodes': list(blockchain.nodes),
    }
    return jsonify(response), 201


@app.route('/nodes/resolve', methods=['GET'])
def consensus():
    """Consensus algorithm endpoint"""
    replaced = blockchain.resolve_conflicts()

    if replaced:
        response = {
            'message': 'Our chain was replaced',
            'new_chain': blockchain.chain
        }
    else:
        response = {
            'message': 'Our chain is authoritative',
            'chain': blockchain.chain
        }

    return jsonify(response), 200


if __name__ == '__main__':
    from argparse import ArgumentParser

    parser = ArgumentParser()
    parser.add_argument('-p', '--port', default=5000, type=int, help='port to listen on')
    args = parser.parse_args()
    port = args.port

    print(f"🚀 {BLOCKCHAIN_NAME} ({COIN_SYMBOL}) v{VERSION}")
    print(f"⛏️  Mining reward: {blockchain.mining_reward} {COIN_SYMBOL}")
    print(f"🎯 Difficulty: {blockchain.base_difficulty} leading zeros")
    print(f"💰 Max supply: {blockchain.max_supply:,} {COIN_SYMBOL}")
    print(f"🔗 Node identifier: {node_identifier}")
    print(f"🌐 Starting on port {port}...")
    
    app.run(host='0.0.0.0', port=port, debug=True)