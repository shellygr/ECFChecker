// Shelly

package vm

import (
	"bytes"
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"sync"

	"github.com/ethereum/go-ethereum/common"
	//"github.com/ethereum/go-ethereum/common/hexutil"
	GenStack "github.com/golang-collections/collections/stack"
	set "gopkg.in/fatih/set.v0"
)

// DISABLE_CHECKER is compilation-controlled flag for ECF checking
var DISABLE_CHECKER = false

var checkerDebugs = true
var debugLevel = 0

// ImportantDebug will always print an output to stdout
func ImportantDebug(format string, a ...interface{}) (n int, err error) {
	return Debug(0, format, a...)
}

// Debug prints a debug line depending on how the program was compiled and on the severity (=level) of the debug
func Debug(level int, format string, a ...interface{}) (n int, err error) {
	if level == 0 || (checkerDebugs && level <= debugLevel) {
		return fmt.Printf(format+"\n", a...)
	}

	return 0, nil
}

// TheChecker is used by EVM
var theChecker *Checker
var once sync.Once

// TheChecker is for singleton implementation
func TheChecker() *Checker {
	once.Do(func() {
		theChecker = &Checker{}
		theChecker.initChecker()
	})

	return theChecker
}

// Segment is the type for non interrupted traces
type Segment struct {
	contract           common.Address
	depth              int
	prevSegment        *Segment
	indexInTransaction int
	indexInCall        int
	readSet            set.Interface // A set of all read-from locations
	writeSet           set.Interface // A set of all written-to locations

	// For opening segments only - how many times returned to it
	hitOnCallCount int
}

func (s Segment) String() string {
	/* For the information regarding hitOnCallCount to appear, need to add the pointer to the segment into transactionSegments slice, and not a copy.
	In runningSegments we add the pointer, thus the indexInCall is calculated correctly. But we later use transactionSegments which is a slice of
	the segment structs themselves.
	I currently find no other motivation to make transactionSegments a slice of pointers to segemnts.
		Maybe performance? Not sure, and it may be prone to new bugs.
	*/
	// if s.indexInCall == 0 {
	// 	return fmt.Sprintf("{%v %v %v %v(/%v) %v %v}", s.contract.Hex(), s.depth, s.indexInTransaction, s.indexInCall, s.hitOnCallCount, s.readSet.String(), s.writeSet.String())
	// }

	return fmt.Sprintf("{%v %v %v %v %v %v}", s.contract.Hex(), s.depth, s.indexInTransaction, s.indexInCall, s.readSet.String(), s.writeSet.String())
}

// Checker is the type of the to-be-generic checker
type Checker struct {
	transactionSegments []Segment
	runningSegments     *GenStack.Stack

	numberOfSegments int

	isRealCall bool
	evmStack   *GenStack.Stack

	TransactionID int
	origin        *common.Address
	blockNumber   *big.Int
	time          *big.Int

	processTime time.Time

	// DB Handler
	dbHandler *sql.DB

	numOfTransactionsCheckedSoFar int
}

// SetDbHandler allows to set the db handler from anywhere
func (checker *Checker) SetDbHandler(db *sql.DB) {
	checker.dbHandler = db
}

func (checker *Checker) initChecker() {
	checker.evmStack = GenStack.New()
	checker.runningSegments = GenStack.New()

	DISABLE_CHECKER = (os.Getenv("EVM_DISABLE_ECF_CHECK") == "1")
	ImportantDebug("Disable ECF Checker is set to: %v", DISABLE_CHECKER)
	if DISABLE_CHECKER {
		ImportantDebug("ECF CHECKER IS DISABLED !!!")
	} else {
		ImportantDebug("ECF Check is in place!")
	}

	debugLevelStr := os.Getenv("EVM_MONITOR_DEBUG_LEVEL")
	if debugLevelStr != "" {
		ImportantDebug("Debug level set to %s", debugLevelStr)
		debugLevelInt, _ := strconv.Atoi(debugLevelStr)
		debugLevel = debugLevelInt
	}
}

func (checker *Checker) GetLastSegment() *Segment {
	return &(checker.transactionSegments[len(checker.transactionSegments)-1])
}

func (checker *Checker) PushNewSegmentFromStart(contract *Contract) {
	segment := Segment{}
	if checker.runningSegments.Len() == 0 {
		segment = Segment{contract: contract.Address(),
			depth:              1,
			prevSegment:        nil,
			readSet:            set.New(),
			writeSet:           set.New(),
			indexInTransaction: 0,
			indexInCall:        0,
			hitOnCallCount:     0}
		checker.transactionSegments = make([]Segment, 0)
	} else {
		segment = Segment{contract: contract.Address(),
			depth:              checker.GetLastSegment().depth + 1,
			prevSegment:        checker.GetLastSegment(),
			readSet:            set.New(),
			writeSet:           set.New(),
			indexInTransaction: checker.numberOfSegments,
			indexInCall:        0,
			hitOnCallCount:     0}
	}

	Debug(3, "Adding segment %v, also to running segments stack. EVM stack %v (%v), isRealCall %v", segment, checker.evmStack, checker.evmStack.Len(), checker.isRealCall)

	checker.transactionSegments = append(checker.transactionSegments, segment)
	checker.numberOfSegments++
	checker.runningSegments.Push(&segment)
}

func (checker *Checker) PushNewSegmentFromEnd() {
	(checker.runningSegments.Peek()).(*Segment).hitOnCallCount++

	segment := Segment{contract: (checker.runningSegments.Peek()).(*Segment).contract,
		depth:              (checker.runningSegments.Peek()).(*Segment).depth,
		prevSegment:        checker.GetLastSegment(),
		readSet:            set.New(),
		writeSet:           set.New(),
		indexInTransaction: checker.numberOfSegments,
		indexInCall:        (checker.runningSegments.Peek()).(*Segment).hitOnCallCount}

	Debug(3, "Adding segment %v", segment)

	checker.transactionSegments = append(checker.transactionSegments, segment)
	checker.numberOfSegments++
}

func GetProjectedTrace(segments []Segment, contract *common.Address) []Segment {
	projection := make([]Segment, 0)
	for i := range segments {
		if bytes.Equal((segments[i].contract.Bytes()), contract.Bytes()) {
			projection = append(projection, segments[i])
		}
	}

	return projection
}

func hasRecursion(projectedSegments []Segment) bool { // O(n)
	if len(projectedSegments) == 0 {
		return false
	}

	// Loop on calls (defined as opening + closing segment indices). Search inside call for higher depth segment with indexInCall==0
	for i := range projectedSegments {
		if isOpeningSegment(projectedSegments[i]) {
			closingIndex := findMatchingClosingSegment(i, projectedSegments) // O(n)
			call := projectedSegments[i : closingIndex+1]
			// Now search inside the call
			for j := range call {
				if call[j].indexInCall == 0 && call[j].depth > projectedSegments[i].depth {
					return true
				}
			} // If not found, we could technically continue to closingIndex+1
		}
	}

	return false
}

func isOpeningSegment(segment Segment) bool {
	return segment.indexInCall == 0
}

// Returns a slice of segments which all pertain to the same call, with the opening segment starting in idx
func findAllSegmentsOfCall(idx int, trace []Segment) []Segment {
	if len(trace) <= idx {
		Debug(1, "Error in findAllSegmentsOfCall, expecting a trace of size > %v", idx)
		return nil
	}

	if !isOpeningSegment(trace[idx]) {
		Debug(1, "Error in findAllSegmentsOfCall, expected %v segment in trace to be an opening segment", idx)
		return nil
	}

	openingSegment := trace[idx]
	suffixContainingCall := trace[idx:]
	call := make([]Segment, 0)
	call = append(call, openingSegment)

	for i := range suffixContainingCall { // 0-idx elements not interesting - it is the opening segment
		if i > 0 { // 0'th element is idx element
			if suffixContainingCall[i].depth == openingSegment.depth {
				if suffixContainingCall[i].indexInCall != 0 {
					call = append(call, suffixContainingCall[i])
				} else {
					return call
				}
			}
		}
	}

	return call
}

// Returns the index in trace of the closing segment matching openingSegment
func findMatchingClosingSegment(idx int, trace []Segment) int { // Consider unifying with findAllSegmentsOfCall
	if len(trace) <= idx {
		Debug(1, "Error in findMatchingClosingSegment, expecting a trace of size > %v, got %v", idx, trace)
		return -1
	}

	if !isOpeningSegment(trace[idx]) {
		Debug(1, "Error in findMatchingClosingSegment, expected %v segment in trace to be an opening segment", idx)
		return -1
	}

	openingSegment := trace[idx]

	candidateIndex := 0
	suffix := trace[idx:]
	for i := range suffix { // 0-idx elements not interesting - it is the opening segment
		if i > 0 { // 0'th element is idx element
			Debug(5, "Opening segment %v, current segment %v", openingSegment, suffix[i])
			if suffix[i].depth == openingSegment.depth {
				if suffix[i].indexInCall != 0 {
					candidateIndex = i
				} else {
					// Found another opening call in the same depth, meaning we can return our candidate index
					return idx + candidateIndex
				}
			}

			if suffix[i].depth < openingSegment.depth { // Back to a previous call - lower depth
				return idx + candidateIndex
			}
		}
	}

	// If candidate index was not updated, this means there were no other segments for that call and openingSegment == closingSegment
	return idx + candidateIndex
}

// Return the index of the second opening segment in this trace (assuming the first segment is an opening segment). It returns 0 otherwise
func findNextOpeningSegment(trace []Segment) int {
	if len(trace) == 0 {
		Debug(1, "Error in findNextOpeningSegment, expecting a trace of size > 0")
		return -1
	}

	if !isOpeningSegment(trace[0]) {
		Debug(1, "Error in findNextOpeningSegment, expected the first segment in the trace to be an opening segment")
	}

	for i := range trace {
		if i > 0 {
			if isOpeningSegment(trace[i]) {
				return i
			}
		}
	}

	return 0
}

// Returns the index of the opening segment which, together with its closing segment and all segments in-between, make for the minimal recursive subtrace in trace.
// Returns 0 if there are no recursive subtraces (or if in 0 we find the minimal recursive subtrace)
func findMinimalRecursiveSubTrace(trace []Segment) int { // O(n^2)
	if len(trace) == 0 {
		Debug(1, "Error in findMinimalRecursiveSubTrace, expecting a trace of size > 0")
		return -1
	}

	if !isOpeningSegment(trace[0]) {
		Debug(1, "Error in findMinimalRecursiveSubTrace, expected the first segment in the trace to be an opening segment - %v", trace)
	}

	candidateTrace := trace[0 : findMatchingClosingSegment(0, trace)+1]
	Debug(3, "candidate trace is %v (%v), trace is %v (%v)", candidateTrace, len(candidateTrace), trace, len(trace))
	if len(candidateTrace) == 0 {
		Debug(1, "Error in findMinimalRecursiveSubTrace, candidate trace may not be empty")
		return -1 // THIS IS AN ERROR!
	}
	if hasRecursion(candidateTrace) {
		nextOpeningSegmentIdx := findNextOpeningSegment(candidateTrace)
		nextOpeningSegmentsCloseIdx := findMatchingClosingSegment(nextOpeningSegmentIdx, trace)
		subtrace := candidateTrace[nextOpeningSegmentIdx : nextOpeningSegmentsCloseIdx+1]
		indexOfMinimalRecursiveSubtrace := findMinimalRecursiveSubTrace(subtrace)
		if indexOfMinimalRecursiveSubtrace == 0 && !hasRecursion(subtrace) { // In 0 we have a recursive subtrace where in 1 we do not. Thus 0 is start of a minimal recursive subtrace
			return 0
		}

		// Otherwise, a minimal recursive subtrace starts at offset of next opening segment + the found index relative to the next opening segment
		return nextOpeningSegmentIdx + indexOfMinimalRecursiveSubtrace

	}

	// If there are no more calls to that contract in the trace
	if len(candidateTrace) == len(trace) {
		return 0
	}

	suffix := trace[len(candidateTrace):]
	Debug(3, "working on suffix of trace: %v", suffix)
	minimalRecursiveSubtraceInSuffix := findMinimalRecursiveSubTrace(suffix)

	if minimalRecursiveSubtraceInSuffix == 0 && !hasRecursion(suffix) {
		return 0 // There is no recursion at all in this case!
	}

	// Otherwise, add to the found index the offset of the suffix
	return len(candidateTrace) + minimalRecursiveSubtraceInSuffix
}

func findAndRemoveOmittables(trace []Segment) []Segment {
	// For each opening segment fetch its call. If no interferences (recursion), check if the unified write set is empty.
	skippedIndices := make([]int, 0) // Keeps an even number of ints, where the first in each pair marks the start of the range to be skipped, and the second is the end.

	for i := range trace {
		if isOpeningSegment(trace[i]) {
			closingSegmentIdx := findMatchingClosingSegment(i, trace)
			subtrace := trace[i : closingSegmentIdx+1]
			if !hasRecursion(subtrace) { // Only if the subtrace containing the call has no recursion, we may check the call
				call := findAllSegmentsOfCall(0, subtrace)
				Debug(3, "findAndRemoveOmittables: trace is %v, subtrace %v, call %v, i %v, closingSegmentIndex %v", trace, subtrace, call, i, closingSegmentIdx)
				isOmittableCall := true
				for j := range call {
					if !call[j].writeSet.IsEmpty() { // Check if any of the call's segments has non-empty write set. If yes, it is not omittable and we break
						isOmittableCall = false
						break
					}
				}

				if isOmittableCall {
					// rebuild subtrace. The assumption is that a call has no recursion only if all its segments are adjacent. So just verify:
					if len(call) == len(subtrace) { // In this case we can just skip this subtrace.
						skippedIndices = append(skippedIndices, i, closingSegmentIdx)
					}
				}
			}
		}
	}

	// Now rebuild trace based on skippedIndices
	newTrace := make([]Segment, 0)
	skippedIndex := 0
	doneWithSkipping := false

	if skippedIndex >= len(skippedIndices) {
		doneWithSkipping = true
	}

	for i := range trace {

		if doneWithSkipping {
			newTrace = append(newTrace, trace[i])
		} else {
			if i > skippedIndices[skippedIndex+1] {
				skippedIndex += 2
			}

			if skippedIndex >= len(skippedIndices) {
				doneWithSkipping = true
			}

			Debug(3, "Skipped indices %v, skippedIndex %v, i %v, newTrace %v", skippedIndices, skippedIndex, i, newTrace)
			if doneWithSkipping || !(skippedIndices[skippedIndex] <= i && i <= skippedIndices[skippedIndex+1]) { // Do not skip
				newTrace = append(newTrace, trace[i])
			}
		}
	}

	Debug(2, "Removed omittables from trace %v (%v), got %v (%v)", trace, len(trace), newTrace, len(newTrace))

	return newTrace
}

func checkLeftMove(segment Segment, prevReadSet set.Interface, prevWriteSet set.Interface) bool {
	/* Condition 1: readset of segment and previous writeset are disjoint (segment not affected by previous segments), and writeset of segment and previous readset are disjoint (previous not affected by segment)
	R(s) \cap W(prev) = \emptyset \land W(s) \cap R(prev) = \emptyset
	*/
	cond1 := (set.Intersection(segment.readSet, prevWriteSet)).IsEmpty() && (set.Intersection(segment.writeSet, prevReadSet)).IsEmpty()

	/* Condition 2: readset of segment and previous writeset are disjoint (segment not affected by previous segments), and writeset of segment and previous writeset are disjoint (even if read values were affected, previous writes are not affected by segment)
	R(s) \cap W(prev) = \emptyset \land W(s) \cap W(prev) = \emptyset
	*/
	// cond2 := (set.Intersection(segment.readSet, prevWriteSet)).IsEmpty() && (set.Intersection(segment.writeSet, prevWriteSet)).IsEmpty() // This is wrong. It may affect the reads that move to the right.

	/* Condition 3: writeset of segment is contained in previous writeset, and writeset of segment and previous readset are disjoint (previous not affected by segment)
	W(s) \subset W(s') \land W(s) \cap R(prev) = \emptyset
	*/
	// cond3 := (prevWriteSet.IsSubset(segment.writeSet)) /* W(s)<prevWriteSet */ && (set.Intersection(segment.writeSet, prevReadSet)).IsEmpty() // this is wrong too!

	Debug(2, "checkLeftMove: Segment %v, Previous read set %v, Previous write set %v, cond1 = %v", segment, prevReadSet, prevWriteSet, cond1)

	return cond1
}

func findCutpoint(trace []Segment, baseDepth int) int {
	Debug(2, "Finding cutpoint for %v with baseDepth %v", trace, baseDepth)
	success := false
	cutpoint := -1
	for cutpoint = len(trace); cutpoint > 0; cutpoint-- { // Guessing the cutpoint
		foundViolation := false

		prefixReadSet := set.New()
		prefixWriteSet := set.New()
		postCutpoint := trace[cutpoint:]
		for idx := range postCutpoint {
			// right-move all inner segments. This is the same as left-moving all outer segments. So keep prefix of inners
			if postCutpoint[idx].depth > baseDepth { // inner call Segment
				prefixReadSet.Merge(postCutpoint[idx].readSet)
				prefixWriteSet.Merge(postCutpoint[idx].writeSet)
			} else {
				if !checkLeftMove(postCutpoint[idx], prefixReadSet, prefixWriteSet) {
					foundViolation = true
					break
				}
			}
		}

		// Continue to left-move inners after successfully right-moved the inners
		if !foundViolation {
			// left-move all inner segments
			prefixReadSet = set.New()
			prefixWriteSet = set.New()
			for idx := range trace[:cutpoint] {
				if trace[idx].depth == baseDepth { // outer call Segment
					prefixReadSet.Merge(trace[idx].readSet)
					prefixWriteSet.Merge(trace[idx].writeSet)
				} else {
					// Check current index with respect to prefix.
					if !checkLeftMove(trace[idx], prefixReadSet, prefixWriteSet) {
						foundViolation = true
						break
					}
				}
			}
		}

		if !foundViolation {
			success = true
			break
		} else {
			Debug(2, "Cutpoint at %v failed, proceeding to next one...", cutpoint)
		}
	}

	if success {
		Debug(2, "Cutpoint is %v", cutpoint)
		return cutpoint
	}

	return -1
}

func attemptToRemoveRecursion(trace []Segment) ([]Segment, bool) {
	// Get the segments of the outermost call.
	outerCall := findAllSegmentsOfCall(0, trace)
	outerCallDepth := outerCall[0].depth

	// Find the cutpoint
	cutpoint := findCutpoint(trace, outerCallDepth)
	if cutpoint == -1 {
		return nil, false
	}

	// Now rebuild segment using the found cutpoint
	newTrace := make([]Segment, 0)
	before := make([]Segment, 0)
	after := make([]Segment, 0)

	for i := range trace {
		if trace[i].depth > outerCallDepth {
			if i < cutpoint {
				before = append(before, trace[i])
			} else { // i >= cutpoint
				after = append(after, trace[i])
			}
		} else if trace[i].depth < outerCallDepth {
			Debug(1, "Error in attemptToRemoveRecursion: when rebuilding the trace, there can't be a lower depth then the outermost call depth")
			return nil, false
		} // else: equal depths. take directly from outercall
	}

	// To fix according to finalized definition, need to merge segments pertaining to the same call which are contnuous

	newTrace = append(append(append(newTrace, before...), outerCall...), after...)

	return newTrace, true
}

func reportNonReentrant(segment Segment, traceLen int) {
	checker := TheChecker()

	stmt := fmt.Sprintf("insert into NON_REENTRANT_TRACE(id, origin, block, time, contract, depth, start_index, length) VALUES(%d, '%s', %d, %d, '%s', %d, %d, %d)",
		checker.TransactionID,
		checker.origin.Hex(),
		checker.blockNumber,
		checker.time,
		segment.contract.Hex(),
		segment.depth,
		segment.indexInTransaction,
		traceLen)

	_, dberr := checker.dbHandler.Exec(stmt)
	if dberr != nil {
		ImportantDebug("Failed to execute %s, %v", stmt, dberr)
	}
}

func checkTraceForReentrancy(trace []Segment) {
	for hasRecursion(trace) {
		trace = findAndRemoveOmittables(trace)

		// If trace is entirely omittable, return. It is obviously reentrant
		if len(trace) == 0 || !hasRecursion(trace) {
			Debug(2, "Transaction is ECF after removing omittables.")
			return
		}

		minimalRecursiveSubTraceOpenIdx := findMinimalRecursiveSubTrace(trace)
		minimalRecursiveSubTraceCloseIdx := findMatchingClosingSegment(minimalRecursiveSubTraceOpenIdx, trace)

		minimalRecursiveSubTrace := trace[minimalRecursiveSubTraceOpenIdx : minimalRecursiveSubTraceCloseIdx+1]
		before := trace[0:minimalRecursiveSubTraceOpenIdx]
		after := make([]Segment, 0)
		if len(trace) > minimalRecursiveSubTraceCloseIdx {
			after = trace[minimalRecursiveSubTraceCloseIdx+1:]
		}

		Debug(3, "Found minimal recursive subtrace %v in idx %v-%v of original trace %v (before %v, after %v)", minimalRecursiveSubTrace, minimalRecursiveSubTraceOpenIdx, minimalRecursiveSubTraceCloseIdx, trace, before, after)

		if !hasRecursion(minimalRecursiveSubTrace) {
			Debug(1, "Error in checkTraceForReentrancy - must have recursion in subtrace in this step - 1 %v", minimalRecursiveSubTrace)
			return
		}

		reorderedSubTrace, success := attemptToRemoveRecursion(minimalRecursiveSubTrace)
		if !success {
			firstSegment := minimalRecursiveSubTrace[0]
			ImportantDebug("Transaction is not ECF! Contract %v, depth %v, index in transaction starting at %v", firstSegment.contract.Hex(), firstSegment.depth, firstSegment.indexInTransaction)

			reportNonReentrant(firstSegment, len(minimalRecursiveSubTrace))

			return
		} else {
			Debug(2, "Subtrace is ECF. Original: %v, Reordered : %v", minimalRecursiveSubTrace, reorderedSubTrace)
		}

		newTrace := make([]Segment, 0)
		newTrace = append(append(append(newTrace, before...), reorderedSubTrace...), after...)
		trace = newTrace
	}

}

func (checker *Checker) checkForReentrancy() {
	checkedContracts := make(map[string]bool)

	Debug(2, "Transaction segments: (%v) %v", len(checker.transactionSegments), checker.transactionSegments)
	if len(checker.transactionSegments) == 1 { // If there is just 1 segment in the transaction, no point in checking it! Optimization
		return
	}

	for i := range checker.transactionSegments {
		contract := checker.transactionSegments[i].contract

		if !checkedContracts[contract.Hex()] {
			projection := GetProjectedTrace(checker.transactionSegments, &contract)
			Debug(2, "Checking contract %v, projection: %v (%v)", contract.Hex(), projection, len(projection))
			checkTraceForReentrancy(projection)
			checkedContracts[contract.Hex()] = true
		}
	}
}

// UponEVMStart is called each time the EVM is run (due to a call or otherwise)
func (checker *Checker) UponEVMStart(evm *Interpreter, contract *Contract) {
	if DISABLE_CHECKER {
		return
	}

	Debug(5, "EVM was Run %s", "")
	checker.numOfTransactionsCheckedSoFar++
	if checker.numOfTransactionsCheckedSoFar%10000 == 0 {
		Debug(1, "Checked %d transactions so far in this run", checker.numOfTransactionsCheckedSoFar)
	}

	if checker.TransactionID == 0 {
		// Read from database
		var lastTransactionID int
		qErr := checker.dbHandler.QueryRow("select txId from LAST_TRANSACTION_ID").Scan(&lastTransactionID)
		if qErr != nil {
			ImportantDebug("Failed to get last tx id")
		} else {
			checker.TransactionID = lastTransactionID
			Debug(1, "Got transaction ID from last run: %v", checker.TransactionID)
		}
	}

	checker.TransactionID++

	// When the call is from a regular user, i.e. when the call stack is empty/quiescent state, we also record the origin, block number, and time
	if checker.evmStack.Len() == 0 {
		checker.origin = &evm.env.Origin
		checker.blockNumber = evm.env.BlockNumber
		checker.time = evm.env.Time
		checker.processTime = time.Now()
	}

	// Create a new Segment
	if checker.evmStack.Len() == 0 || checker.isRealCall {
		checker.PushNewSegmentFromStart(contract)
	}

	checker.evmStack.Push(checker.evmStack.Len() == 0 || checker.isRealCall)

	// Reset isRealCall until the next CALL opcode is seen
	checker.isRealCall = false
}

// UponEVMEnd is called each time the EVM run's ends (due to a return or otherwise)
func (checker *Checker) UponEVMEnd(evm *Interpreter, contract *Contract) {
	if DISABLE_CHECKER {
		return
	}

	// We pop from running segments only if the evmStack top is true (i.e. a real call, and not a delegated one)
	activeCallIsARealCall := checker.evmStack.Pop().(bool)
	Debug(5, "Finished a real call? %v", activeCallIsARealCall)
	if checker.runningSegments.Len() > 1 && activeCallIsARealCall == true {
		checker.runningSegments.Pop()
		checker.PushNewSegmentFromEnd()
	}

	if checker.runningSegments.Len() == 1 && activeCallIsARealCall {
		FirstSegment := checker.runningSegments.Pop().(*Segment)

		Debug(2, "Transaction ended (Block #%v, contract %v). Checking if ECF with respect to all participating contracts.", evm.env.BlockNumber, FirstSegment.contract.Hex())

		reentrancyCheckStartTime := time.Now()
		checker.checkForReentrancy()
		reentrancyCheckDuration := time.Since(reentrancyCheckStartTime)
		totalProcessDuration := time.Since(checker.processTime)
		Debug(2, "Reentrancy check (Block #%v, contract %v) took %s / %s total", evm.env.BlockNumber, FirstSegment.contract.Hex(), reentrancyCheckDuration, totalProcessDuration)
	}

	if checker.evmStack.Len() == 0 {
		checker.origin = nil
		checker.blockNumber = nil
		checker.time = nil
		checker.numberOfSegments = 0
		// checker.processTime = nil // will be reset properly when we return from quiescent state and run another transaction
	}
}

// UponSStore is called upon each SSTORE opcode called
func (checker *Checker) UponSStore(evm *EVM, contract *Contract, loc common.Hash, val *big.Int) {
	if DISABLE_CHECKER {
		return
	}

	if storageDebug {
		Debug(6, "SSTORE contract %v, location %v and value %v\n", contract.Address().Hex(), loc, val)
	}

	checker.GetLastSegment().writeSet.Add(loc)
}

// UponSLoad is called upon each SLOAD opcode called
func (checker *Checker) UponSLoad(evm *EVM, contract *Contract, loc common.Hash, val *big.Int) {
	if DISABLE_CHECKER {
		return
	}

	if storageDebug {
		Debug(6, "SLOAD contract %v, location %v and value %v\n", contract.Address().Hex(), loc, val)
	}

	// Add only if not in writeSet.
	ws := checker.GetLastSegment().writeSet
	if ws.Has(loc) {
		Debug(2, "Read location %v already appears in writeset %v", loc, ws)
	}

	checker.GetLastSegment().readSet.Add(loc)
}

// UponCall is called upon each CALL opcode called
func (checker *Checker) UponCall(evm *EVM, contract *Contract, callee common.Address, value *big.Int, input []byte) {
	if DISABLE_CHECKER {
		return
	}

	checker.isRealCall = true
}
