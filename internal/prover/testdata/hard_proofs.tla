-- Derived from https://github.com/tlaplus/CommunityModules/ and abides
-- to its MIT license terms.
-- Module with hard proofs that can fail.
--------------------- MODULE hard_proofs ---------------------
EXTENDS Integers, Sequences

===============================================================================
-- Hard predicates
===============================================================================

-- Prime number predicate
IsPrime(n) ==
    n >= 2 /\ \A d \in 2..n-1 : ~(d | n)

-- Perfect square predicate
IsPerfectSquare(n) ==
    \E k \in Nat : n = k * k

-- Fibonacci sequence
Fib(0) == 0
Fib(1) == 1
Fib(n) == Fib(n-1) + Fib(n-2)

===============================================================================
-- Theorems (passing)
===============================================================================

<1>1. THEOREM PrimeTwo ==
    IsPrime(2)
BY DEF IsPrime

<1>2. THEOREM PrimeThree ==
    IsPrime(3)
BY DEF IsPrime

<1>3. THEOREM PerfectSquareFour ==
    IsPerfectSquare(4)
BY DEF IsPerfectSquare

<1>4. THEOREM FibonacciZero ==
    Fib(0) = 0
BY DEF Fib

<1>5. THEOREM FibonacciOne ==
    Fib(1) = 1
BY DEF Fib

<1>6. THEOREM FibonacciTwo ==
    Fib(2) = 1
BY DEF Fib

===============================================================================
-- A theorem that FAILS: claims all primes are odd
-- (2 is prime but not odd)
<1>7. THEOREM AllPrimesAreOdd ==
    \A p \in 2..10 : IsPrime(p) => ~(p = 2)
BY DEF IsPrime

===============================================================================
