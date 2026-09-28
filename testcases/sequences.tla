-- Derived from https://github.com/tlaplus/CommunityModules/ and abides
-- to its MIT license terms.
-- Sequences module with proof steps.
----------------------- MODULE sequences -----------------------
EXTENDS Integers, Sequences

===============================================================================
-- Basic sequence operations
===============================================================================

Append(s, x) == AppendTail(s, <<x>>)

Head(s) == s[1]

Tail(s) == SubSeq(s, 2, Len(s))

===============================================================================
-- Theorems about sequences
===============================================================================

THEOREM LenAppend ==
    \A s \in Seq(Int), x \in Int :
        Len(Append(s, x)) = Len(s) + 1
BY DEF Append, Len

THEOREM HeadAppend ==
    \A s \in Seq(Int), x \in Int :
        Head(<<x>>) = x
BY DEF Head, Append

THEOREM TailAppend ==
    \A s \in Seq(Int), x \in Int :
        Tail(<<x>>) = <<>>
BY DEF Tail, Append

THEOREM AppendIdempotent ==
    \A s \in Seq(Int), x \in Int :
        Len(Append(s, x)) > 0
        => Head(Append(s, x)) = x
BY DEF Append, Head

===============================================================================
