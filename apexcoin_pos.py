import hashlib
import json
from time import time
from urllib.parse import urlparse
from uuid import uuid4
import base64
from Crypto.PublicKey import RSA
from Crypto.Signature import pkcs1_15
from Crypto.Hash import SHA256
import random

import requests
from flask import Flask, jsonify, request
from flask_cors import CORS

# ApexCoin PoS Configuration
BLOCKCHAIN_NAME = "ApexCoin"
COIN_SYMBOL = "APEX"
VERSION = "2.0.0-PoS"

# PoS Parameters
INITIAL_SUPPLY = 21_000_000
MIN_VALIDATOR_STAKE = 1000  # Minimum stake to become validator
VALIDATOR_COMMISSION_RATE = 0.10  # 10% commission on rewards
EPOCH_LENGTH = 100  # Blocks per epoch
BLOCK_TIME_TARGET = 12  # Target seconds per block
SLASHING_PENALTY = 0.05  # 5% stake slashed for misbehavior
OFFLINE_PENALTY = 0.01  # 1% per epoch offline
MIN_DELEGATION = 10  # Minimum delegation amount
UNBONDING_PERIOD = 7 * 24 * 3600  # 7 days in seconds
ANNUAL_INFLATION = 0.05  # 5% annual inflation
FINALITY_THRESHOLD = 0.67  # 67% of stake for finality


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
        """Sign transaction with private key"""
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


class Validator:
    """Validator node in PoS system"""
    
    def __init__(self, address, stake):
        self.address = address
        self.stake = stake
        self.delegated_stake = 0
        self.delegators = {}  # address -> amount
        self.blocks_produced = 0
        self.blocks_missed = 0
        self.is_active = True
        self.last_active_epoch = 0
        self.commission_rate = VALIDATOR_COMMISSION_RATE
        self.total_rewards = 0
        self.slashing_history = []
        self.reputation_score = 100
        self.joined_epoch = 0
        
    def total_stake(self):
        """Total stake including delegations"""
        return self.stake + self.delegated_stake
    
    def add_delegation(self, delegator, amount):
        """Add delegation from user"""
        self.delegators[delegator] = self.delegators.get(delegator, 0) + amount
        self.delegated_stake += amount
        
    def remove_delegation(self, delegator, amount):
        """Remove delegation"""
        if delegator in self.delegators:
            current = self.delegators[delegator]
            remove_amount = min(amount, current)
            self.delegators[delegator] -= remove_amount
            self.delegated_stake -= remove_amount
            if self.delegators[delegator] <= 0:
                del self.delegators[delegator]
            return remove_amount
        return 0
    
    def slash(self, percentage, reason):
        """Slash validator stake"""
        slash_amount = self.stake * percentage
        self.stake -= slash_amount
        self.slashing_history.append({
            'time': time(),
            'amount': slash_amount,
            'reason': reason
        })
        self.reputation_score = max(0, self.reputation_score - 20)
        return slash_amount
    
    def to_dict(self):
        """Convert to dictionary"""
        return {
            'address': self.address,
            'stake': self.stake,
            'delegated_stake': self.delegated_stake,
            'total_stake': self.total_stake(),
            'blocks_produced': self.blocks_produced,
            'blocks_missed': self.blocks_missed,
            'is_active': self.is_active,
            'commission_rate': self.commission_rate,
            'total_rewards': self.total_rewards,
            'reputation_score': self.reputation_score,
            'delegators_count': len(self.delegators)
        }


class GovernanceProposal:
    """Governance proposal for protocol changes"""
    
    def __init__(self, proposer, title, description, proposal_type, parameters):
        self.id = hashlib.sha256(f"{proposer}{title}{time()}".encode()).hexdigest()[:16]
        self.proposer = proposer
        self.title = title
        self.description = description
        self.type = proposal_type  # 'parameter', 'upgrade', 'funding'
        self.parameters = parameters
        self.votes_for = 0
        self.votes_against = 0
        self.voters = {}  # address -> weight
        self.status = 'active'  # active, passed, rejected, executed
        self.created_time = time()
        self.voting_end_time = time() + (7 * 24 * 3600)  # 7 days
        self.execution_time = None
        
    def vote(self, voter, weight, vote_for):
        """Record a vote"""
        if voter not in self.voters:
            self.voters[voter] = {'weight': weight, 'vote': vote_for}
            if vote_for:
                self.votes_for += weight
            else:
                self.votes_against += weight
            return True
        return False
    
    def finalize(self, quorum_threshold=0.33, approval_threshold=0.51):
        """Finalize voting and determine outcome"""
        if time() < self.voting_end_time:
            return False
            
        total_votes = self.votes_for + self.votes_against
        
        # Check quorum
        if total_votes < quorum_threshold:
            self.status = 'rejected'
            return True
            
        # Check approval
        if self.votes_for / total_votes >= approval_threshold:
            self.status = 'passed'
            self.execution_time = time() + (2 * 24 * 3600)  # 2 day timelock
        else:
            self.status = 'rejected'
            
        return True
    
    def to_dict(self):
        """Convert to dictionary"""
        return {
            'id': self.id,
            'proposer': self.proposer,
            'title': self.title,
            'description': self.description,
            'type': self.type,
            'parameters': self.parameters,
            'votes_for': self.votes_for,
            'votes_against': self.votes_against,
            'status': self.status,
            'created_time': self.created_time,
            'voting_end_time': self.voting_end_time,
            'execution_time': self.execution_time,
            'total_voters': len(self.voters)
        }


class ApexCoinPoSBlockchain:
    """ApexCoin Proof-of-Stake Blockchain"""
    
    def __init__(self):
        self.current_transactions = []
        self.chain = []
        self.nodes = set()
        self.wallets = {}
        
        # PoS State
        self.validators = {}  # address -> Validator
        self.current_epoch = 0
        self.epoch_start_time = time()
        self.total_staked = 0
        self.unbonding_queue = []  # List of unbonding requests
        
        # Governance
        self.proposals = {}  # proposal_id -> GovernanceProposal
        self.governance_power = {}  # address -> voting power
        
        # Economic parameters
        self.total_supply = INITIAL_SUPPLY
        self.inflation_rate = ANNUAL_INFLATION
        
        # Performance tracking
        self.validator_performance = {}
        self.finalized_blocks = []
        
        # Create genesis block with initial distribution
        self.new_block(previous_hash='0', proof=100, validator_address='genesis')
        self._distribute_genesis()

    def _distribute_genesis(self):
        """Distribute initial supply from genesis"""
        # Reserve for staking rewards
        self.wallets['genesis_reserve'] = INITIAL_SUPPLY

    def register_validator(self, address, stake_amount):
        """Register a new validator"""
        if stake_amount < MIN_VALIDATOR_STAKE:
            raise ValueError(f"Minimum stake required: {MIN_VALIDATOR_STAKE}")
            
        if self.get_balance(address) < stake_amount:
            raise ValueError("Insufficient balance to stake")
            
        if address in self.validators:
            raise ValueError("Already a validator")
        
        # Lock stake
        self.wallets[address] = self.get_balance(address) - stake_amount
        
        # Create validator
        validator = Validator(address, stake_amount)
        validator.joined_epoch = self.current_epoch
        self.validators[address] = validator
        self.total_staked += stake_amount
        
        return validator

    def delegate_stake(self, delegator, validator_address, amount):
        """Delegate stake to a validator"""
        if amount < MIN_DELEGATION:
            raise ValueError(f"Minimum delegation: {MIN_DELEGATION}")
            
        if self.get_balance(delegator) < amount:
            raise ValueError("Insufficient balance")
            
        if validator_address not in self.validators:
            raise ValueError("Validator does not exist")
            
        validator = self.validators[validator_address]
        if not validator.is_active:
            raise ValueError("Validator is not active")
        
        # Lock delegation
        self.wallets[delegator] = self.get_balance(delegator) - amount
        validator.add_delegation(delegator, amount)
        self.total_staked += amount
        
        return True

    def undelegate_stake(self, delegator, validator_address, amount):
        """Start undelegation process (unbonding period applies)"""
        if validator_address not in self.validators:
            raise ValueError("Validator does not exist")
            
        validator = self.validators[validator_address]
        removed = validator.remove_delegation(delegator, amount)
        
        if removed == 0:
            raise ValueError("No delegation found or insufficient amount")
        
        # Add to unbonding queue
        unbonding_entry = {
            'delegator': delegator,
            'amount': removed,
            'completion_time': time() + UNBONDING_PERIOD,
            'validator': validator_address
        }
        self.unbonding_queue.append(unbonding_entry)
        self.total_staked -= removed
        
        return unbonding_entry

    def process_unbonding(self):
        """Process completed unbonding requests"""
        current_time = time()
        completed = []
        
        for entry in self.unbonding_queue[:]:
            if current_time >= entry['completion_time']:
                # Return funds
                delegator = entry['delegator']
                amount = entry['amount']
                self.wallets[delegator] = self.get_balance(delegator) + amount
                completed.append(entry)
                self.unbonding_queue.remove(entry)
        
        return completed

    def select_validator(self, epoch=None):
        """Select validator based on stake-weighted probability"""
        if not self.validators:
            return None
            
        # Filter active validators
        active_validators = {
            addr: val for addr, val in self.validators.items()
            if val.is_active and val.total_stake() >= MIN_VALIDATOR_STAKE
        }
        
        if not active_validators:
            return None
        
        # Calculate total active stake
        total_active_stake = sum(v.total_stake() for v in active_validators.values())
        
        if total_active_stake == 0:
            return None
        
        # Stake-weighted random selection
        rand_point = random.uniform(0, total_active_stake)
        cumulative = 0
        
        for address, validator in active_validators.items():
            cumulative += validator.total_stake()
            if rand_point <= cumulative:
                return address
        
        # Fallback
        return list(active_validators.keys())[0]

    def slash_validator(self, validator_address, reason):
        """Slash validator for misbehavior"""
        if validator_address not in self.validators:
            return None
            
        validator = self.validators[validator_address]
        slashed_amount = validator.slash(SLASHING_PENALTY, reason)
        
        # Deactivate if stake too low
        if validator.stake < MIN_VALIDATOR_STAKE:
            validator.is_active = False
        
        return {
            'validator': validator_address,
            'slashed_amount': slashed_amount,
            'reason': reason,
            'remaining_stake': validator.stake
        }

    def calculate_block_reward(self):
        """Calculate block reward based on inflation"""
        blocks_per_year = (365 * 24 * 3600) / BLOCK_TIME_TARGET
        reward_per_block = (self.total_supply * self.inflation_rate) / blocks_per_year
        return reward_per_block

    def distribute_rewards(self, validator_address, block_reward, tx_fees):
        """Distribute block rewards to validator and delegators"""
        if validator_address not in self.validators:
            return None
            
        validator = self.validators[validator_address]
        total_reward = block_reward + tx_fees
        
        # Validator commission
        validator_commission = total_reward * validator.commission_rate
        remaining_reward = total_reward - validator_commission
        
        # Distribute to validator based on their own stake
        validator_share = (validator.stake / validator.total_stake()) * remaining_reward
        validator.total_rewards += validator_commission + validator_share
        self.wallets[validator_address] = self.get_balance(validator_address) + validator_commission + validator_share
        
        # Distribute to delegators
        distribution = {}
        for delegator, delegation_amount in validator.delegators.items():
            delegator_share = (delegation_amount / validator.total_stake()) * remaining_reward
            self.wallets[delegator] = self.get_balance(delegator) + delegator_share
            distribution[delegator] = delegator_share
        
        # Mint new tokens (inflation)
        self.total_supply += block_reward
        
        return {
            'total_reward': total_reward,
            'validator_commission': validator_commission,
            'validator_share': validator_share,
            'delegator_distribution': distribution
        }

    def create_proposal(self, proposer, title, description, proposal_type, parameters):
        """Create a governance proposal"""
        # Check if proposer has minimum stake
        min_proposal_stake = self.total_staked * 0.01  # 1% of total stake
        proposer_power = self.get_voting_power(proposer)
        
        if proposer_power < min_proposal_stake:
            raise ValueError(f"Insufficient voting power to create proposal. Need: {min_proposal_stake}")
        
        proposal = GovernanceProposal(proposer, title, description, proposal_type, parameters)
        self.proposals[proposal.id] = proposal
        
        return proposal

    def vote_on_proposal(self, proposal_id, voter, vote_for):
        """Vote on a governance proposal"""
        if proposal_id not in self.proposals:
            raise ValueError("Proposal not found")
            
        proposal = self.proposals[proposal_id]
        
        if proposal.status != 'active':
            raise ValueError("Proposal is not active")
        
        # Get voting power
        voting_power = self.get_voting_power(voter)
        
        if voting_power == 0:
            raise ValueError("No voting power")
        
        success = proposal.vote(voter, voting_power, vote_for)
        
        if not success:
            raise ValueError("Already voted")
        
        return True

    def get_voting_power(self, address):
        """Calculate voting power for an address"""
        power = 0
        
        # Own stake as validator
        if address in self.validators:
            power += self.validators[address].stake
        
        # Delegated stake
        for validator in self.validators.values():
            if address in validator.delegators:
                power += validator.delegators[address]
        
        return power

    def finalize_proposals(self):
        """Finalize all completed proposals"""
        finalized = []
        
        for proposal in self.proposals.values():
            if proposal.status == 'active':
                total_stake = self.total_staked
                quorum = total_stake * 0.33
                
                if proposal.finalize(quorum_threshold=quorum):
                    finalized.append(proposal)
        
        return finalized

    def execute_proposal(self, proposal_id):
        """Execute a passed proposal after timelock"""
        if proposal_id not in self.proposals:
            raise ValueError("Proposal not found")
            
        proposal = self.proposals[proposal_id]
        
        if proposal.status != 'passed':
            raise ValueError("Proposal not passed")
            
        if time() < proposal.execution_time:
            raise ValueError("Timelock not expired")
        
        # Execute based on type
        if proposal.type == 'parameter':
            # Update blockchain parameters
            for key, value in proposal.parameters.items():
                if hasattr(self, key):
                    setattr(self, key, value)
        
        proposal.status = 'executed'
        return True

    def process_epoch(self):
        """Process epoch changes"""
        current_block = len(self.chain)
        
        if current_block % EPOCH_LENGTH == 0 and current_block > 0:
            self.current_epoch += 1
            self.epoch_start_time = time()
            
            # Check validator performance
            for address, validator in self.validators.items():
                if validator.is_active:
                    # Penalize offline validators
                    if validator.last_active_epoch < self.current_epoch - 1:
                        offline_penalty = validator.stake * OFFLINE_PENALTY
                        validator.stake -= offline_penalty
                        validator.reputation_score = max(0, validator.reputation_score - 5)
                        
                        if validator.stake < MIN_VALIDATOR_STAKE:
                            validator.is_active = False
            
            # Process unbonding
            self.process_unbonding()
            
            # Finalize proposals
            self.finalize_proposals()
            
            return True
        
        return False

    def new_block(self, proof, previous_hash, validator_address='genesis'):
        """Create new block"""
        block = {
            'index': len(self.chain) + 1,
            'timestamp': time(),
            'transactions': self.current_transactions,
            'proof': proof,
            'previous_hash': previous_hash or self.hash(self.chain[-1]),
            'validator': validator_address,
            'epoch': self.current_epoch,
            'attestations': []
        }

        # Calculate and distribute rewards
        if validator_address != 'genesis':
            block_reward = self.calculate_block_reward()
            tx_fees = sum(tx.get('fee', 0) for tx in self.current_transactions)
            
            reward_distribution = self.distribute_rewards(validator_address, block_reward, tx_fees)
            block['reward_distribution'] = reward_distribution
            
            # Update validator stats
            if validator_address in self.validators:
                self.validators[validator_address].blocks_produced += 1
                self.validators[validator_address].last_active_epoch = self.current_epoch

        # Update balances
        for transaction in self.current_transactions:
            sender = transaction['sender']
            recipient = transaction['recipient']
            amount = transaction['amount']
            
            if sender != "0":
                self.wallets[sender] = self.get_balance(sender) - amount
            
            self.wallets[recipient] = self.get_balance(recipient) + amount

        self.current_transactions = []
        self.chain.append(block)
        
        # Process epoch if needed
        self.process_epoch()
        
        return block

    def new_transaction(self, sender, recipient, amount, signature=None, fee=0):
        """Create new transaction"""
        if sender != "0":
            total_cost = amount + fee
            if self.get_balance(sender) < total_cost:
                raise ValueError(f"Insufficient balance")
            
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

    @property
    def last_block(self):
        return self.chain[-1]

    @staticmethod
    def hash(block):
        """Create SHA-256 hash of block"""
        block_string = json.dumps(block, sort_keys=True).encode()
        return hashlib.sha256(block_string).hexdigest()

    def proof_of_stake(self, last_block):
        """Simpler PoS proof (just a formality since validator is pre-selected)"""
        last_proof = last_block['proof']
        last_hash = self.hash(last_block)
        
        proof = 0
        while not self.valid_proof(last_proof, proof, last_hash):
            proof += 1
        
        return proof

    @staticmethod
    def valid_proof(last_proof, proof, last_hash):
        """Simple validation (lighter than PoW)"""
        guess = f'{last_proof}{proof}{last_hash}'.encode()
        guess_hash = hashlib.sha256(guess).hexdigest()
        return guess_hash[:2] == "00"  # Much easier than PoW


# Flask Application
app = Flask(__name__)
CORS(app)

node_identifier = str(uuid4()).replace('-', '')
blockchain = ApexCoinPoSBlockchain()
created_wallets = {}


@app.route('/info', methods=['GET'])
def blockchain_info():
    """Get blockchain information"""
    response = {
        'name': BLOCKCHAIN_NAME,
        'symbol': COIN_SYMBOL,
        'version': VERSION,
        'consensus': 'Proof-of-Stake',
        'total_supply': blockchain.total_supply,
        'total_staked': blockchain.total_staked,
        'staking_ratio': blockchain.total_staked / blockchain.total_supply if blockchain.total_supply > 0 else 0,
        'inflation_rate': blockchain.inflation_rate,
        'blocks': len(blockchain.chain),
        'current_epoch': blockchain.current_epoch,
        'active_validators': len([v for v in blockchain.validators.values() if v.is_active]),
        'total_validators': len(blockchain.validators),
        'nodes': len(blockchain.nodes)
    }
    return jsonify(response), 200


@app.route('/validators', methods=['GET'])
def get_validators():
    """Get all validators"""
    validators = [v.to_dict() for v in blockchain.validators.values()]
    validators.sort(key=lambda x: x['total_stake'], reverse=True)
    
    response = {
        'validators': validators,
        'total_validators': len(validators),
        'active_validators': len([v for v in validators if v['is_active']]),
        'total_staked': blockchain.total_staked
    }
    return jsonify(response), 200


@app.route('/validators/register', methods=['POST'])
def register_validator():
    """Register as a validator"""
    values = request.get_json()
    
    required = ['address', 'stake_amount']
    if not all(k in values for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    
    try:
        validator = blockchain.register_validator(values['address'], values['stake_amount'])
        
        response = {
            'message': 'Validator registered successfully',
            'validator': validator.to_dict(),
            'minimum_stake': MIN_VALIDATOR_STAKE
        }
        return jsonify(response), 201
        
    except ValueError as e:
        return jsonify({'error': str(e)}), 400


@app.route('/delegate', methods=['POST'])
def delegate():
    """Delegate stake to a validator"""
    values = request.get_json()
    
    required = ['delegator', 'validator_address', 'amount']
    if not all(k in values for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    
    try:
        blockchain.delegate_stake(
            values['delegator'],
            values['validator_address'],
            values['amount']
        )
        
        validator = blockchain.validators[values['validator_address']]
        
        response = {
            'message': 'Delegation successful',
            'delegator': values['delegator'],
            'validator': values['validator_address'],
            'amount': values['amount'],
            'validator_total_stake': validator.total_stake()
        }
        return jsonify(response), 200
        
    except ValueError as e:
        return jsonify({'error': str(e)}), 400


@app.route('/undelegate', methods=['POST'])
def undelegate():
    """Undelegate stake from a validator"""
    values = request.get_json()
    
    required = ['delegator', 'validator_address', 'amount']
    if not all(k in values for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    
    try:
        unbonding = blockchain.undelegate_stake(
            values['delegator'],
            values['validator_address'],
            values['amount']
        )
        
        response = {
            'message': 'Undelegation initiated',
            'unbonding': unbonding,
            'unbonding_period_seconds': UNBONDING_PERIOD,
            'completion_date': unbonding['completion_time']
        }
        return jsonify(response), 200
        
    except ValueError as e:
        return jsonify({'error': str(e)}), 400


@app.route('/mine', methods=['POST'])
def mine():
    """Mine a new block (PoS style)"""
    values = request.get_json() if request.method == 'POST' else {}
    
    # Select validator
    validator_address = blockchain.select_validator()
    
    if not validator_address:
        return jsonify({'error': 'No active validators available'}), 400
    
    last_block = blockchain.last_block
    proof = blockchain.proof_of_stake(last_block)
    
    # Create block
    previous_hash = blockchain.hash(last_block)
    block = blockchain.new_block(proof, previous_hash, validator_address)
    
    response = {
        'message': f'⛓️ New Block Forged (PoS)',
        'block': {
            'index': block['index'],
            'timestamp': block['timestamp'],
            'transactions': len(block['transactions']),
            'validator': block['validator'],
            'epoch': block['epoch'],
        },
        'reward_distribution': block.get('reward_distribution'),
        'validator_info': blockchain.validators[validator_address].to_dict()
    }
    return jsonify(response), 200


@app.route('/governance/proposals', methods=['GET', 'POST'])
def governance_proposals():
    """Get all proposals or create new one"""
    if request.method == 'GET':
        proposals = [p.to_dict() for p in blockchain.proposals.values()]
        proposals.sort(key=lambda x: x['created_time'], reverse=True)
        
        return jsonify({
            'proposals': proposals,
            'total': len(proposals)
        }), 200
    
    else:  # POST
        values = request.get_json()
        
        required = ['proposer', 'title', 'description', 'type', 'parameters']
        if not all(k in values for k in required):
            return jsonify({'error': 'Missing required fields'}), 400
        
        try:
            proposal = blockchain.create_proposal(
                values['proposer'],
                values['title'],
                values['description'],
                values['type'],
                values['parameters']
            )
            
            return jsonify({
                'message': 'Proposal created',
                'proposal': proposal.to_dict()
            }), 201
            
        except ValueError as e:
            return jsonify({'error': str(e)}), 400


@app.route('/governance/vote', methods=['POST'])
def vote():
    """Vote on a proposal"""
    values = request.get_json()
    
    required = ['proposal_id', 'voter', 'vote_for']
    if not all(k in values for k in required):
        return jsonify({'error': 'Missing required fields'}), 400
    
    try:
        blockchain.vote_on_proposal(
            values['proposal_id'],
            values['voter'],
            values['vote_for']
        )
        
        proposal = blockchain.proposals[values['proposal_id']]
        
        return jsonify({
            'message': 'Vote recorded',
            'proposal': proposal.to_dict()
        }), 200
        
    except ValueError as e:
        return jsonify({'error': str(e)}), 400


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
        'warning': '⚠️ SAVE YOUR PRIVATE KEY!'
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


@app.route('/chain', methods=['GET'])
def full_chain():
    """Get the full blockchain"""
    response = {
        'chain': blockchain.chain,
        'length': len(blockchain.chain),
    }
    return jsonify(response), 200


if __name__ == '__main__':
    from argparse import ArgumentParser

    parser = ArgumentParser()
    parser.add_argument('-p', '--port', default=5000, type=int, help='port to listen on')
    args = parser.parse_args()
    port = args.port

    print(f"🚀 {BLOCKCHAIN_NAME} ({COIN_SYMBOL}) v{VERSION}")
    print(f"⚡ Consensus: Proof-of-Stake")
    print(f"💰 Total Supply: {INITIAL_SUPPLY:,} {COIN_SYMBOL}")
    print(f"📊 Inflation Rate: {ANNUAL_INFLATION * 100}% annually")
    print(f"🔒 Min Validator Stake: {MIN_VALIDATOR_STAKE:,} {COIN_SYMBOL}")
    print(f"🔗 Node identifier: {node_identifier}")
    print(f"🌐 Starting on port {port}...")
    
    app.run(host='0.0.0.0', port=port, debug=True)
