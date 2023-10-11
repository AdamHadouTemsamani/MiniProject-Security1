# At the bottom can be found a main method that runs all methods and shows the results.

# Exercise 1 - Write code that allows Alice to build an encrypted message containing '2000'

generator = 666
prime = 6661
public_key = 2227

alice_privatekey = 1234  # I have chosen the private key '1234', but anything can be put here.
message = 2000

def encrypt(x, y, prime, generator, message):
    gy = (generator ** y) % prime
    gxy = (x ** y) % prime
    c = (gxy * message) % prime
    return gy, c

# Exercise 2 - Write code that allows Eve to find Bob’s private key and reconstruct Alice’s message.

def findPrivateKey(g, p, pk):
    for i in range(p):
        gy = (g ** i) % p
        if pk == gy:
            return i
    return 0

def reconstructMessage(gy, c):
    inverse = pow(gy, -(findPrivateKey(generator, prime, public_key)), prime)
    return (c * inverse) % prime


# Exercise 3 - Write code that allows Weave to modify Alice’s encrypted message so that
# when Bob decrypts it, he will get the double amount originally sent from Alice

def modifyMessage(c):
    return c * 2

def main():
    print("=== Mini Project 1 ===")
    print('')

    #Exercise 1
    gy, c = encrypt(public_key, alice_privatekey, prime, generator, message)
    print('Exercise 1: Running encrypt()')
    print(f'gy: {gy}, c: {c}')
    print('')

    #Exercise 2
    privateKey = findPrivateKey(generator, prime, public_key)
    reconstructedMessage = reconstructMessage(gy, c)
    print('Exercise 2: Running findPrivateKey() and reconstructMessage()')
    print(f'Private key: {privateKey}')
    print(f'Reconstructed message: {reconstructedMessage}')
    print('')

    #Exercise 3
    reconstructedMessage1 = reconstructMessage(gy, c)
    print('Exercise 3: Running with and without modifyMessage()')
    print(f'Normal message: {reconstructedMessage1}')

    reconstructedMessage2 = reconstructMessage(gy, modifyMessage(c))
    print(f'Modified message: {reconstructedMessage2}')
    print('')

if __name__ == "__main__":
    main()