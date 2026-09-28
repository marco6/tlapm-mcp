-- Derived from https://github.com/tlaplus/CommunityModules/ and abides
-- to its MIT license terms.
-- Module with both passing and failing proof steps.
----------------------- MODULE failing -----------------------
EXTENDS Integers, Sequences

===============================================================================
-- Definitions
===============================================================================

-- A recursive function for computing factorials
Factorial(n) ==
    IF n = 0 THEN 1
    ELSE n * Factorial(n - 1)
    END

-- Predicate for prime numbers
IsPrime(n) ==
    n >= 2 /\ \A d \in 2..n-1 : ~(d | n)

-- A more complex predicate
Divides(a, b) ==
    \E k \in Int : b = a * k

===============================================================================
-- Theorems (all pass)
===============================================================================

THEOREM FactorialZero ==
    Factorial(0) = 1
BY DEF Factorial

THEOREM FactorialOne ==
    Factorial(1) = 1
BY DEF Factorial

THEOREM FactorialTwo ==
    Factorial(2) = 2
BY DEF Factorial

THEOREM DividesReflexive ==
    \A n \in Nat : Divides(n, n)
BY DEF Divides

===============================================================================
-- A theorem that FAILS: claims factorial grows monotonically
-- This fails because we need to prove it for all n, not just specific cases
THEOREM FactorialGrows ==
    \A n \in Nat : Factorial(n) >= n
BY DEF Factorial, Divides

===============================================================================
