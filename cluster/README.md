For the theory, please consult [Clustering by Compression](https://arxiv.org/pdf/cs/0312044.pdf) by Rudi Cilibrasi and Paul Vitanyi,
as well as [course slides](http://www.hutter1.net/ai/spredict.pdf) by Marcus Hutter.
Mammals DNA copied from the examples folder of [CompLearn](https://complearn.org/index.html)

## Run.
```
rm -rf dna
mkdir dna
go run atcg.go -d dna -s mammals
go run compute.go -i gzip -d dna
go run compute.go -d dna
```
