-- Derived from https://github.com/tlaplus/CommunityModules/ and abides
-- to its MIT license terms.
-- Folds module with folding operations.
---------------------- MODULE folding ----------------------
EXTENDS Integers, FiniteSets, Sequences

===============================================================================
-- Folding operations
===============================================================================

FoldSet(S, f) ==
    IF S = {} THEN
        f({}, {})
    ELSE
        \E x \in S :
            LET R = S \ {x}
            IN FoldSet(R, f)
            /\ f(R, S)
    END

FoldSeq(s, f) ==
    IF s = <<>> THEN
        f(<<>>, <<>>)
    ELSE
        LET r = DropTail(s)
        IN FoldSeq(r, f)
        /\ f(r, s)
    END

===============================================================================
-- Theorems about folds
===============================================================================

THEOREM FoldSetEmpty ==
    FoldSet({}, f) = f({}, {})
BY DEF FoldSet

THEOREM FoldSeqSingleton ==
    \A x : FoldSeq(<<x>>, f) = f(<<>>, <<x>>)
BY DEF FoldSeq

THEOREM FoldSeqAppend ==
    \A s \in Seq(Int), x \in Int :
        FoldSeq(<<x>>, f) /\ Len(s) > 0
        => \E y \in s : y = Head(s)
BY DEF FoldSeq, Head, Len

===============================================================================
