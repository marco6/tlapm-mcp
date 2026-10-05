---- MODULE arithmetic_theorem ----
EXTENDS Integers, TLAPS

CommutativeSum(x, y) == x + y = y + x

THEOREM SumCommutes ==
    \A x, y \in Int : CommutativeSum(x, y)
BY DEF CommutativeSum

===========================
