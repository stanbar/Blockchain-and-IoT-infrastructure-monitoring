# Privacy-Preserving, Scalable Blockchain-Based Solution for Monitoring Industrial Infrastructure in the Near Real-Time

## Authors

### Stanislaw Baranski

Department of Electronic, Telecommunication and Informatics, Gdańsk University of Technology

stanislaw.baranski@pg.edu.pl

### Andrzej Sobecki

Department of Electronic, Telecommunication and Informatics, Gdańsk University of Technology

andrzej.sobecki@pg.edu.pl

### Julian Szymanski

Department of Electronic, Telecommunication and Informatics, Gdańsk University of Technology

julian.szymanski@eti.pg.edu.pl

## Abstract

This paper proposes an improved monitoring and measuring system dedicated to industrial infrastructure. Our model achieves security of data by incorporating cryptographical methods and near real-time access by the use of virtual tree structure over records. The currently available blockchain networks are not very well adapted to tasks related to the continuous monitoring of the parameters of industrial installations. In the database systems delivered by default (the so-called world state), only the resultant or the last value recorded by the IoT device is stored. Effective use of measurement values recorded in the past requires each time viewing the entire chain of recorded events for a given IoT device. The solution proposed in the article introduces the concept of dependent wallets, the purpose of which is the aggregation and indexation of changes in machine parameters, recorded in the original wallets. As a result, we can easily get data from a certain sensor or sensors in the specified date range, even if the chain of transactions is very long. Our contribution is a universal mechanism that improves the efficiency of the infrastructure monitoring process, which uses blockchains to record measurements from sensors. The proposed model has been experimentally tested on two types of blockchains: Stellar and Hyperledger Fabric.

**Keywords**: blockchain; Hyperledger Fabric; Stellar; IoT


[Read more](https://www.mdpi.com/2076-3417/12/14/7143/htm)
