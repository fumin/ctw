Context Tree Weighting
-----

## Documentation
Package ctw provides an implementation of the Context Tree Weighting algorithm. Also contained is an implementation of the Rissanen-Langdon Arithmetic Coding algorithm, which is combined with Context Tree Weighting to create a lossless compression/decompression utility.

Below is an example of using this package to compress Lincoln's Gettysburg address:

```
go run compress/main.go gettysburg.txt > gettys.ctw
cat gettys.ctw | go run decompress/main.go > gettys.dctw
diff gettysburg.txt gettys.dctw
```

The results are noticeably superior to that of other commercial applications on a Mac OS X:
  * Original: 1463
  * tar.gz: 993
  * 7z: 908
  * zip: 874
  * CTW: 772

Reference: F.M.J. Willems and Tj. J. Tjalkens, Complexity Reduction of the Context-Tree Weighting Algorithm: A Study for KPN Research, Technical University of Eindhoven, EIDMA Report RS.97.01.

Full documentation at https://godoc.org/github.com/fumin/ctw.

## License
Please note that both the Context Tree Weighting and Arithmetic Coding algorithms are PATENTED.
Therefore, this project should only be used for academic, and never for commercial purposes.

## Testing
`go test`

## Questions
* Why does increasing the depth above 48 not improve the compression of gettysburg.txt? Depth 48 gives 772 bytes, while depth 60 also gives 772 bytes.
* The exposition in https://cs.anu.edu.au/courses/comp4620/2015/slides-ctw.pdf gives a CTW based way of predicting the next bit. However, it is not clear how should we predict the next say 10 bits, without iterating through the 1024 different possibilities.
