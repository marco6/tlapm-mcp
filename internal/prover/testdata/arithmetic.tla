-- Derived from https://github.com/tlaplus/CommunityModules/ and abides
-- to its MIT license terms.
-- Arithmetic module with passing and failing theorems.
----------------------------- MODULE arithmetic -----------------------------
EXTENDS Integers

===============================================================================
-- Basic predicates
===============================================================================

IsEven(n) == \E k \in Int : n = 2 * k

IsOdd(n) == \E k \in Int : n = 2 * k + 1

Sum(n) == IF n = 0 THEN 0 ELSE n + Sum(n - 1)

===============================================================================
-- Theorems about arithmetic
===============================================================================

THEOREM EvenPlusEvenIsEven ==
    \A n, m \in Int :
        /\ IsEven(n)
        /\ IsEven(m)
        => IsEven(n + m)
BY DEF IsEven

THEOREM OddPlusOddIsEven ==
    \A n, m \in Int :
        /\ IsOdd(n)
        /\ IsOdd(m)
        => IsEven(n + m)
BY DEF IsOdd, IsEven

THEOREM EvenNotOdd ==
    \A n \in Int :
        ~ (IsEven(n) /\ IsOdd(n))
BY DEF IsEven, IsOdd

THEOREM SumEqualsZero ==
    Sum(0) = 0
BY DEF Sum

-- A theorem that fails: claims even + odd = even (false)
THEOREM EvenPlusOddIsEven ==
    \A n, m \in Int :
        /\ IsEven(n)
        /\ IsOdd(m)
        => IsEven(n + m)
BY DEF IsEven, IsOdd

===============================================================================
