# Exercise 1 - Write code that allows Alice to build an encrypted message containing '2000'

generator = 666
prime = 6661
public_key = 2227

alice_privatekey = 1234  # I have chosen '1234', but anything can be put here
message = 2000

def encrypt(x, y, prime, generator, message):
    gy = (generator ** y) % prime
    gxy = (x ** y) % prime
    c = (gxy * message) % prime
    return gy, c


gy, c = encrypt(public_key, alice_privatekey, prime, generator, message)
print(f'gy: {gy}, c: {c}')


# Exercise 2 - Write code that allows Eve to find Bob’s private key and reconstruct Alice’s message.

def findPrivateKey(g, p, pk):
    for i in range(p):
        gy = (g ** i) % p
        if pk == gy:
            return i
    return 0

privateKey = findPrivateKey(generator, prime, public_key)
print(f'private key: {privateKey}')

def reconstructMessage(gy, c):
    inverse = pow(gy, -privateKey, prime)
    return (c * inverse) % prime

reconstructedMessage = reconstructMessage(gy, c)
print(f'reconstructed message: {reconstructedMessage}')

# Exercise 3 - Write code that allows Weave to modify Alice’s encrypted message so that
# when Bob decrypts it, he will get the double amount originally sent from Alice

def modifyMessage(c):
    return c * 2

reconstructedMessage1 = reconstructMessage(gy, c)
print(f'normal message: {reconstructedMessage1}')

reconstructedMessage2 = reconstructMessage(gy, modifyMessage(c))
print(f'modified message: {reconstructedMessage2}')

def main():
    print("Hello this is main!")

if __name__ == "__main__":
    main()