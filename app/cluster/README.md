This project demonstrates the use of compression to do science, such as studying mammalian evolution and constructing the phelogeny of the SARS virus. More details can be found in this [slide](https://docs.google.com/presentation/d/1LUbo-6mLpYTwcELOLlRR4ohku9j2kCiQj_2sYPh0uWA/edit?usp=sharing)

For the theory, please consult [Clustering by Compression](https://arxiv.org/pdf/cs/0312044.pdf) by Rudi Cilibrasi and Paul Vitanyi,
as well as [course slides](http://www.hutter1.net/ai/spredict.pdf) by Marcus Hutter.

## Run.
```
go run compute.go -d mammals
go run compute.go -i gzip -d mammals
```
