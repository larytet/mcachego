package hashtable

type ModuloSize func(uint64) int

// Using 2^n-1 I have 20% collisions rate
// A real prime does not improve much
// See https://stackoverflow.com/questions/21854191/generating-prime-numbers-in-go
// https://github.com/agis/gofool/blob/master/atkin.go
func GetPower2Sub1(N int) int {
	return GetPower2(N) - 1
}

// Straignt from https://stackoverflow.com/questions/466204/rounding-up-to-next-power-of-2
// A naive "for i < N i = i << 1" would do the trick as well, but I feel luck today
// I wonder if the compiler understands the naive version and the assembly ends up with
// shifst and ORs anyway.
func GetPower2(N int) int {
	N--
	N |= N >> 1
	N |= N >> 2
	N |= N >> 4
	N |= N >> 8
	N |= N >> 16
	N |= N >> 32
	N++
	return N
}

// I want the compiler to optimize modulo by generating the best assembler it can
// See also https://probablydance.com/2017/02/26/i-wrote-the-fastest-hashtable/
// and source code https://github.com/skarupke/flat_hash_map/blob/master/flat_hash_map.hpp
// This function shaves 10% off the Store() CPU consumption
// Call to moduloSize() is 4x faster than a naive hash % size - 4ns vs 8ns
// Can I do it faster by approximating the result? Does it make sense to cache results
// of modulo for small tables?
// In C++ I would use a generic
func getModuloSizeFunction(size int) ModuloSize {
	switch size {
	// between 5K to 1M entries are at the top of the swicth
	// TODO check what the compiler does here
	case 2:
		return moduloSize_2
	case 3:
		return moduloSize_3
	case 5:
		return moduloSize_5
	case 7:
		return moduloSize_7
	case 11:
		return moduloSize_11
	case 13:
		return moduloSize_13
	case 17:
		return moduloSize_17
	case 23:
		return moduloSize_23
	case 29:
		return moduloSize_29
	case 37:
		return moduloSize_37
	case 47:
		return moduloSize_47
	case 59:
		return moduloSize_59
	case 73:
		return moduloSize_73
	case 97:
		return moduloSize_97
	case 127:
		return moduloSize_127
	case 151:
		return moduloSize_151
	case 197:
		return moduloSize_197
	case 251:
		return moduloSize_251
	case 313:
		return moduloSize_313
	case 397:
		return moduloSize_397
	case 499:
		return moduloSize_499
	case 631:
		return moduloSize_631
	case 797:
		return moduloSize_797
	case 1009:
		return moduloSize_1009
	case 1259:
		return moduloSize_1259
	case 1597:
		return moduloSize_1597
	case 2011:
		return moduloSize_2011
	case 2539:
		return moduloSize_2539
	case 3203:
		return moduloSize_3203
	case 4027:
		return moduloSize_4027
	case 5087:
		return moduloSize_5087
	case 6421:
		return moduloSize_6421
	case 8089:
		return moduloSize_8089
	case 10193:
		return moduloSize_10193
	case 12853:
		return moduloSize_12853
	case 16193:
		return moduloSize_16193
	case 20399:
		return moduloSize_20399
	case 25717:
		return moduloSize_25717
	case 32401:
		return moduloSize_32401
	case 40823:
		return moduloSize_40823
	case 51437:
		return moduloSize_51437
	case 64811:
		return moduloSize_64811
	case 81649:
		return moduloSize_81649
	case 102877:
		return moduloSize_102877
	case 129607:
		return moduloSize_129607
	case 163307:
		return moduloSize_163307
	case 205759:
		return moduloSize_205759
	case 259229:
		return moduloSize_259229
	case 326617:
		return moduloSize_326617
	case 411527:
		return moduloSize_411527
	case 518509:
		return moduloSize_518509
	case 653267:
		return moduloSize_653267
	case 823117:
		return moduloSize_823117
	case 1037059:
		return moduloSize_1037059
	case 1306601:
		return moduloSize_1306601
	case 1646237:
		return moduloSize_1646237
	case 2074129:
		return moduloSize_2074129
	case 2613229:
		return moduloSize_2613229
	case 3292489:
		return moduloSize_3292489
	case 4148279:
		return moduloSize_4148279
	case 5226491:
		return moduloSize_5226491
	case 6584983:
		return moduloSize_6584983
	case 8296553:
		return moduloSize_8296553
	case 10453007:
		return moduloSize_10453007
	case 13169977:
		return moduloSize_13169977
	case 16593127:
		return moduloSize_16593127
	case 20906033:
		return moduloSize_20906033
	case 26339969:
		return moduloSize_26339969
	case 33186281:
		return moduloSize_33186281
	case 41812097:
		return moduloSize_41812097
	case 52679969:
		return moduloSize_52679969
	case 66372617:
		return moduloSize_66372617
	case 83624237:
		return moduloSize_83624237
	case 105359939:
		return moduloSize_105359939
	case 132745199:
		return moduloSize_132745199
	case 167248483:
		return moduloSize_167248483
	case 210719881:
		return moduloSize_210719881
	case 265490441:
		return moduloSize_265490441
	case 334496971:
		return moduloSize_334496971
	case 421439783:
		return moduloSize_421439783
	case 530980861:
		return moduloSize_530980861
	case 668993977:
		return moduloSize_668993977
	case 842879579:
		return moduloSize_842879579
	case 1061961721:
		return moduloSize_1061961721
	case 1337987929:
		return moduloSize_1337987929
	case 1685759167:
		return moduloSize_1685759167
	case 2123923447:
		return moduloSize_2123923447
	case 2675975881:
		return moduloSize_2675975881
	case 3371518343:
		return moduloSize_3371518343
	case 4247846927:
		return moduloSize_4247846927
	case 5351951779:
		return moduloSize_5351951779
	case 6743036717:
		return moduloSize_6743036717
	case 8495693897:
		return moduloSize_8495693897
	case 10703903591:
		return moduloSize_10703903591
	case 13486073473:
		return moduloSize_13486073473
	case 16991387857:
		return moduloSize_16991387857
	case 21407807219:
		return moduloSize_21407807219
	case 26972146961:
		return moduloSize_26972146961
	case 33982775741:
		return moduloSize_33982775741
	case 42815614441:
		return moduloSize_42815614441
	case 53944293929:
		return moduloSize_53944293929
	case 67965551447:
		return moduloSize_67965551447
	case 85631228929:
		return moduloSize_85631228929
	case 107888587883:
		return moduloSize_107888587883
	case 135931102921:
		return moduloSize_135931102921
	case 171262457903:
		return moduloSize_171262457903
	case 215777175787:
		return moduloSize_215777175787
	case 271862205833:
		return moduloSize_271862205833
	case 342524915839:
		return moduloSize_342524915839
	case 431554351609:
		return moduloSize_431554351609
	case 543724411781:
		return moduloSize_543724411781
	case 685049831731:
		return moduloSize_685049831731
	case 863108703229:
		return moduloSize_863108703229
	case 1087448823553:
		return moduloSize_1087448823553
	case 1370099663459:
		return moduloSize_1370099663459
	case 1726217406467:
		return moduloSize_1726217406467
	case 2174897647073:
		return moduloSize_2174897647073
	case 2740199326961:
		return moduloSize_2740199326961
	case 3452434812973:
		return moduloSize_3452434812973
	case 4349795294267:
		return moduloSize_4349795294267
	case 5480398654009:
		return moduloSize_5480398654009
	case 6904869625999:
		return moduloSize_6904869625999
	case 8699590588571:
		return moduloSize_8699590588571
	case 10960797308051:
		return moduloSize_10960797308051
	case 13809739252051:
		return moduloSize_13809739252051
	case 17399181177241:
		return moduloSize_17399181177241
	case 21921594616111:
		return moduloSize_21921594616111
	case 27619478504183:
		return moduloSize_27619478504183
	case 34798362354533:
		return moduloSize_34798362354533
	case 43843189232363:
		return moduloSize_43843189232363
	case 55238957008387:
		return moduloSize_55238957008387
	case 69596724709081:
		return moduloSize_69596724709081
	case 87686378464759:
		return moduloSize_87686378464759
	case 110477914016779:
		return moduloSize_110477914016779
	case 139193449418173:
		return moduloSize_139193449418173
	case 175372756929481:
		return moduloSize_175372756929481
	case 220955828033581:
		return moduloSize_220955828033581
	case 278386898836457:
		return moduloSize_278386898836457
	case 350745513859007:
		return moduloSize_350745513859007
	case 441911656067171:
		return moduloSize_441911656067171
	case 556773797672909:
		return moduloSize_556773797672909
	case 701491027718027:
		return moduloSize_701491027718027
	case 883823312134381:
		return moduloSize_883823312134381
	case 1113547595345903:
		return moduloSize_1113547595345903
	case 1402982055436147:
		return moduloSize_1402982055436147
	case 1767646624268779:
		return moduloSize_1767646624268779
	case 2227095190691797:
		return moduloSize_2227095190691797
	case 2805964110872297:
		return moduloSize_2805964110872297
	case 3535293248537579:
		return moduloSize_3535293248537579
	case 4454190381383713:
		return moduloSize_4454190381383713
	case 5611928221744609:
		return moduloSize_5611928221744609
	case 7070586497075177:
		return moduloSize_7070586497075177
	case 8908380762767489:
		return moduloSize_8908380762767489
	case 11223856443489329:
		return moduloSize_11223856443489329
	case 14141172994150357:
		return moduloSize_14141172994150357
	case 17816761525534927:
		return moduloSize_17816761525534927
	case 22447712886978529:
		return moduloSize_22447712886978529
	case 28282345988300791:
		return moduloSize_28282345988300791
	case 35633523051069991:
		return moduloSize_35633523051069991
	case 44895425773957261:
		return moduloSize_44895425773957261
	case 56564691976601587:
		return moduloSize_56564691976601587
	case 71267046102139967:
		return moduloSize_71267046102139967
	case 89790851547914507:
		return moduloSize_89790851547914507
	case 113129383953203213:
		return moduloSize_113129383953203213
	case 142534092204280003:
		return moduloSize_142534092204280003
	case 179581703095829107:
		return moduloSize_179581703095829107
	case 226258767906406483:
		return moduloSize_226258767906406483
	case 285068184408560057:
		return moduloSize_285068184408560057
	case 359163406191658253:
		return moduloSize_359163406191658253
	case 452517535812813007:
		return moduloSize_452517535812813007
	case 570136368817120201:
		return moduloSize_570136368817120201
	case 718326812383316683:
		return moduloSize_718326812383316683
	case 905035071625626043:
		return moduloSize_905035071625626043
	case 1140272737634240411:
		return moduloSize_1140272737634240411
	case 1436653624766633509:
		return moduloSize_1436653624766633509
	case 1810070143251252131:
		return moduloSize_1810070143251252131
	case 2280545475268481167:
		return moduloSize_2280545475268481167
	case 2873307249533267101:
		return moduloSize_2873307249533267101
	case 3620140286502504283:
		return moduloSize_3620140286502504283
	case 4561090950536962147:
		return moduloSize_4561090950536962147
	case 5746614499066534157:
		return moduloSize_5746614499066534157
	case 7240280573005008577:
		return moduloSize_7240280573005008577
	case 9122181901073924329:
		return moduloSize_9122181901073924329
	}
	// Fail if I do not have a suitable function
	return nil
}

var PrimeList = []int{
	2, 3, 5, 7, 11, 13, 17, 23, 29, 37, 47,
	59, 73, 97, 127, 151, 197, 251, 313, 397,
	499, 631, 797, 1009, 1259, 1597, 2011, 2539,
	3203, 4027, 5087, 6421, 8089, 10193, 12853, 16193,
	20399, 25717, 32401, 40823, 51437, 64811, 81649,
	102877, 129607, 163307, 205759, 259229, 326617,
	411527, 518509, 653267, 823117, 1037059, 1306601,
	1646237, 2074129, 2613229, 3292489, 4148279, 5226491,
	6584983, 8296553, 10453007, 13169977, 16593127, 20906033,
	26339969, 33186281, 41812097, 52679969, 66372617,
	83624237, 105359939, 132745199, 167248483, 210719881,
	265490441, 334496971, 421439783, 530980861, 668993977,
	842879579, 1061961721, 1337987929, 1685759167, 2123923447,
	2675975881, 3371518343, 4247846927, 5351951779, 6743036717,
	8495693897, 10703903591, 13486073473, 16991387857,
	21407807219, 26972146961, 33982775741, 42815614441,
	53944293929, 67965551447, 85631228929, 107888587883,
	135931102921, 171262457903, 215777175787, 271862205833,
	342524915839, 431554351609, 543724411781, 685049831731,
	863108703229, 1087448823553, 1370099663459, 1726217406467,
	2174897647073, 2740199326961, 3452434812973, 4349795294267,
	5480398654009, 6904869625999, 8699590588571, 10960797308051,
	13809739252051, 17399181177241, 21921594616111, 27619478504183,
	34798362354533, 43843189232363, 55238957008387, 69596724709081,
	87686378464759, 110477914016779, 139193449418173,
	175372756929481, 220955828033581, 278386898836457,
	350745513859007, 441911656067171, 556773797672909,
	701491027718027, 883823312134381, 1113547595345903,
	1402982055436147, 1767646624268779, 2227095190691797,
	2805964110872297, 3535293248537579, 4454190381383713,
	5611928221744609, 7070586497075177, 8908380762767489,
	11223856443489329, 14141172994150357, 17816761525534927,
	22447712886978529, 28282345988300791, 35633523051069991,
	44895425773957261, 56564691976601587, 71267046102139967,
	89790851547914507, 113129383953203213, 142534092204280003,
	179581703095829107, 226258767906406483, 285068184408560057,
	359163406191658253, 452517535812813007, 570136368817120201,
	718326812383316683, 905035071625626043, 1140272737634240411,
	1436653624766633509, 1810070143251252131, 2280545475268481167,
	2873307249533267101, 3620140286502504283, 4561090950536962147,
	5746614499066534157, 7240280573005008577, 9122181901073924329} // 11493228998133068689, 14480561146010017169, 18446744073709551557

// From https://github.com/skarupke/flat_hash_map/blob/master/flat_hash_map.hpp
// Prime number for the table size is slighly better than (2^n-1) in some cases
func getSize(N int) int {

	for _, p := range PrimeList {
		if p >= N {
			return p
		}
	}
	// if there is no match in the table of primes I fallback to the size (2^n-1)
	return GetPower2Sub1(N)
}

func moduloSize_2(hash uint64) int {
	return int(hash % 2)
}
func moduloSize_3(hash uint64) int {
	return int(hash % 3)
}
func moduloSize_5(hash uint64) int {
	return int(hash % 5)
}
func moduloSize_7(hash uint64) int {
	return int(hash % 7)
}
func moduloSize_11(hash uint64) int {
	return int(hash % 11)
}
func moduloSize_13(hash uint64) int {
	return int(hash % 13)
}
func moduloSize_17(hash uint64) int {
	return int(hash % 17)
}
func moduloSize_23(hash uint64) int {
	return int(hash % 23)
}
func moduloSize_29(hash uint64) int {
	return int(hash % 29)
}
func moduloSize_37(hash uint64) int {
	return int(hash % 37)
}
func moduloSize_47(hash uint64) int {
	return int(hash % 47)
}

func moduloSize_59(hash uint64) int {
	return int(hash % 59)
}
func moduloSize_73(hash uint64) int {
	return int(hash % 73)
}
func moduloSize_97(hash uint64) int {
	return int(hash % 97)
}
func moduloSize_127(hash uint64) int {
	return int(hash % 127)
}
func moduloSize_151(hash uint64) int {
	return int(hash % 151)
}
func moduloSize_197(hash uint64) int {
	return int(hash % 197)
}
func moduloSize_251(hash uint64) int {
	return int(hash % 251)
}
func moduloSize_313(hash uint64) int {
	return int(hash % 313)
}
func moduloSize_397(hash uint64) int {
	return int(hash % 397)
}

func moduloSize_499(hash uint64) int {
	return int(hash % 499)
}
func moduloSize_631(hash uint64) int {
	return int(hash % 631)
}
func moduloSize_797(hash uint64) int {
	return int(hash % 797)
}
func moduloSize_1009(hash uint64) int {
	return int(hash % 1009)
}
func moduloSize_1259(hash uint64) int {
	return int(hash % 1259)
}
func moduloSize_1597(hash uint64) int {
	return int(hash % 1597)
}
func moduloSize_2011(hash uint64) int {
	return int(hash % 2011)
}
func moduloSize_2539(hash uint64) int {
	return int(hash % 2539)
}

func moduloSize_3203(hash uint64) int {
	return int(hash % 3203)
}
func moduloSize_4027(hash uint64) int {
	return int(hash % 4027)
}
func moduloSize_5087(hash uint64) int {
	return int(hash % 5087)
}
func moduloSize_6421(hash uint64) int {
	return int(hash % 6421)
}
func moduloSize_8089(hash uint64) int {
	return int(hash % 8089)
}
func moduloSize_10193(hash uint64) int {
	return int(hash % 10193)
}
func moduloSize_12853(hash uint64) int {
	return int(hash % 12853)
}
func moduloSize_16193(hash uint64) int {
	return int(hash % 16193)
}

func moduloSize_20399(hash uint64) int {
	return int(hash % 20399)
}
func moduloSize_25717(hash uint64) int {
	return int(hash % 25717)
}
func moduloSize_32401(hash uint64) int {
	return int(hash % 32401)
}
func moduloSize_40823(hash uint64) int {
	return int(hash % 40823)
}
func moduloSize_51437(hash uint64) int {
	return int(hash % 51437)
}
func moduloSize_64811(hash uint64) int {
	return int(hash % 64811)
}
func moduloSize_81649(hash uint64) int {
	return int(hash % 81649)
}

func moduloSize_102877(hash uint64) int {
	return int(hash % 102877)
}
func moduloSize_129607(hash uint64) int {
	return int(hash % 129607)
}
func moduloSize_163307(hash uint64) int {
	return int(hash % 163307)
}
func moduloSize_205759(hash uint64) int {
	return int(hash % 205759)
}
func moduloSize_259229(hash uint64) int {
	return int(hash % 259229)
}
func moduloSize_326617(hash uint64) int {
	return int(hash % 326617)
}

func moduloSize_411527(hash uint64) int {
	return int(hash % 411527)
}
func moduloSize_518509(hash uint64) int {
	return int(hash % 518509)
}
func moduloSize_653267(hash uint64) int {
	return int(hash % 653267)
}
func moduloSize_823117(hash uint64) int {
	return int(hash % 823117)
}
func moduloSize_1037059(hash uint64) int {
	return int(hash % 1037059)
}
func moduloSize_1306601(hash uint64) int {
	return int(hash % 1306601)
}

func moduloSize_1646237(hash uint64) int {
	return int(hash % 1646237)
}
func moduloSize_2074129(hash uint64) int {
	return int(hash % 2074129)
}
func moduloSize_2613229(hash uint64) int {
	return int(hash % 2613229)
}
func moduloSize_3292489(hash uint64) int {
	return int(hash % 3292489)
}
func moduloSize_4148279(hash uint64) int {
	return int(hash % 4148279)
}
func moduloSize_5226491(hash uint64) int {
	return int(hash % 5226491)
}

func moduloSize_6584983(hash uint64) int {
	return int(hash % 6584983)
}
func moduloSize_8296553(hash uint64) int {
	return int(hash % 8296553)
}
func moduloSize_10453007(hash uint64) int {
	return int(hash % 10453007)
}
func moduloSize_13169977(hash uint64) int {
	return int(hash % 13169977)
}
func moduloSize_16593127(hash uint64) int {
	return int(hash % 16593127)
}
func moduloSize_20906033(hash uint64) int {
	return int(hash % 20906033)
}

func moduloSize_26339969(hash uint64) int {
	return int(hash % 26339969)
}
func moduloSize_33186281(hash uint64) int {
	return int(hash % 33186281)
}
func moduloSize_41812097(hash uint64) int {
	return int(hash % 41812097)
}
func moduloSize_52679969(hash uint64) int {
	return int(hash % 52679969)
}
func moduloSize_66372617(hash uint64) int {
	return int(hash % 66372617)
}

func moduloSize_83624237(hash uint64) int {
	return int(hash % 83624237)
}
func moduloSize_105359939(hash uint64) int {
	return int(hash % 105359939)
}
func moduloSize_132745199(hash uint64) int {
	return int(hash % 132745199)
}
func moduloSize_167248483(hash uint64) int {
	return int(hash % 167248483)
}
func moduloSize_210719881(hash uint64) int {
	return int(hash % 210719881)
}

func moduloSize_265490441(hash uint64) int {
	return int(hash % 265490441)
}
func moduloSize_334496971(hash uint64) int {
	return int(hash % 334496971)
}
func moduloSize_421439783(hash uint64) int {
	return int(hash % 421439783)
}
func moduloSize_530980861(hash uint64) int {
	return int(hash % 530980861)
}
func moduloSize_668993977(hash uint64) int {
	return int(hash % 668993977)
}

func moduloSize_842879579(hash uint64) int {
	return int(hash % 842879579)
}
func moduloSize_1061961721(hash uint64) int {
	return int(hash % 1061961721)
}
func moduloSize_1337987929(hash uint64) int {
	return int(hash % 1337987929)
}
func moduloSize_1685759167(hash uint64) int {
	return int(hash % 1685759167)
}
func moduloSize_2123923447(hash uint64) int {
	return int(hash % 2123923447)
}

func moduloSize_2675975881(hash uint64) int {
	return int(hash % 2675975881)
}
func moduloSize_3371518343(hash uint64) int {
	return int(hash % 3371518343)
}
func moduloSize_4247846927(hash uint64) int {
	return int(hash % 4247846927)
}
func moduloSize_5351951779(hash uint64) int {
	return int(hash % 5351951779)
}
func moduloSize_6743036717(hash uint64) int {
	return int(hash % 6743036717)
}

func moduloSize_8495693897(hash uint64) int {
	return int(hash % 8495693897)
}
func moduloSize_10703903591(hash uint64) int {
	return int(hash % 10703903591)
}
func moduloSize_13486073473(hash uint64) int {
	return int(hash % 13486073473)
}
func moduloSize_16991387857(hash uint64) int {
	return int(hash % 16991387857)
}

func moduloSize_21407807219(hash uint64) int {
	return int(hash % 21407807219)
}
func moduloSize_26972146961(hash uint64) int {
	return int(hash % 26972146961)
}
func moduloSize_33982775741(hash uint64) int {
	return int(hash % 33982775741)
}
func moduloSize_42815614441(hash uint64) int {
	return int(hash % 42815614441)
}

func moduloSize_53944293929(hash uint64) int {
	return int(hash % 53944293929)
}
func moduloSize_67965551447(hash uint64) int {
	return int(hash % 67965551447)
}
func moduloSize_85631228929(hash uint64) int {
	return int(hash % 85631228929)
}
func moduloSize_107888587883(hash uint64) int {
	return int(hash % 107888587883)
}

func moduloSize_135931102921(hash uint64) int {
	return int(hash % 135931102921)
}
func moduloSize_171262457903(hash uint64) int {
	return int(hash % 171262457903)
}
func moduloSize_215777175787(hash uint64) int {
	return int(hash % 215777175787)
}
func moduloSize_271862205833(hash uint64) int {
	return int(hash % 271862205833)
}

func moduloSize_342524915839(hash uint64) int {
	return int(hash % 342524915839)
}
func moduloSize_431554351609(hash uint64) int {
	return int(hash % 431554351609)
}
func moduloSize_543724411781(hash uint64) int {
	return int(hash % 543724411781)
}
func moduloSize_685049831731(hash uint64) int {
	return int(hash % 685049831731)
}

func moduloSize_863108703229(hash uint64) int {
	return int(hash % 863108703229)
}
func moduloSize_1087448823553(hash uint64) int {
	return int(hash % 1087448823553)
}
func moduloSize_1370099663459(hash uint64) int {
	return int(hash % 1370099663459)
}
func moduloSize_1726217406467(hash uint64) int {
	return int(hash % 1726217406467)
}

func moduloSize_2174897647073(hash uint64) int {
	return int(hash % 2174897647073)
}
func moduloSize_2740199326961(hash uint64) int {
	return int(hash % 2740199326961)
}
func moduloSize_3452434812973(hash uint64) int {
	return int(hash % 3452434812973)
}
func moduloSize_4349795294267(hash uint64) int {
	return int(hash % 4349795294267)
}

func moduloSize_5480398654009(hash uint64) int {
	return int(hash % 5480398654009)
}
func moduloSize_6904869625999(hash uint64) int {
	return int(hash % 6904869625999)
}
func moduloSize_8699590588571(hash uint64) int {
	return int(hash % 8699590588571)
}
func moduloSize_10960797308051(hash uint64) int {
	return int(hash % 10960797308051)
}

func moduloSize_13809739252051(hash uint64) int {
	return int(hash % 13809739252051)
}
func moduloSize_17399181177241(hash uint64) int {
	return int(hash % 17399181177241)
}
func moduloSize_21921594616111(hash uint64) int {
	return int(hash % 21921594616111)
}
func moduloSize_27619478504183(hash uint64) int {
	return int(hash % 27619478504183)
}

func moduloSize_34798362354533(hash uint64) int {
	return int(hash % 34798362354533)
}
func moduloSize_43843189232363(hash uint64) int {
	return int(hash % 43843189232363)
}
func moduloSize_55238957008387(hash uint64) int {
	return int(hash % 55238957008387)
}
func moduloSize_69596724709081(hash uint64) int {
	return int(hash % 69596724709081)
}

func moduloSize_87686378464759(hash uint64) int {
	return int(hash % 87686378464759)
}
func moduloSize_110477914016779(hash uint64) int {
	return int(hash % 110477914016779)
}
func moduloSize_139193449418173(hash uint64) int {
	return int(hash % 139193449418173)
}

func moduloSize_175372756929481(hash uint64) int {
	return int(hash % 175372756929481)
}
func moduloSize_220955828033581(hash uint64) int {
	return int(hash % 220955828033581)
}
func moduloSize_278386898836457(hash uint64) int {
	return int(hash % 278386898836457)
}

func moduloSize_350745513859007(hash uint64) int {
	return int(hash % 350745513859007)
}
func moduloSize_441911656067171(hash uint64) int {
	return int(hash % 441911656067171)
}
func moduloSize_556773797672909(hash uint64) int {
	return int(hash % 556773797672909)
}

func moduloSize_701491027718027(hash uint64) int {
	return int(hash % 701491027718027)
}
func moduloSize_883823312134381(hash uint64) int {
	return int(hash % 883823312134381)
}
func moduloSize_1113547595345903(hash uint64) int {
	return int(hash % 1113547595345903)
}

func moduloSize_1402982055436147(hash uint64) int {
	return int(hash % 1402982055436147)
}
func moduloSize_1767646624268779(hash uint64) int {
	return int(hash % 1767646624268779)
}
func moduloSize_2227095190691797(hash uint64) int {
	return int(hash % 2227095190691797)
}

func moduloSize_2805964110872297(hash uint64) int {
	return int(hash % 2805964110872297)
}
func moduloSize_3535293248537579(hash uint64) int {
	return int(hash % 3535293248537579)
}
func moduloSize_4454190381383713(hash uint64) int {
	return int(hash % 4454190381383713)
}

func moduloSize_5611928221744609(hash uint64) int {
	return int(hash % 5611928221744609)
}
func moduloSize_7070586497075177(hash uint64) int {
	return int(hash % 7070586497075177)
}
func moduloSize_8908380762767489(hash uint64) int {
	return int(hash % 8908380762767489)
}

func moduloSize_11223856443489329(hash uint64) int {
	return int(hash % 11223856443489329)
}
func moduloSize_14141172994150357(hash uint64) int {
	return int(hash % 14141172994150357)
}
func moduloSize_17816761525534927(hash uint64) int {
	return int(hash % 17816761525534927)
}

func moduloSize_22447712886978529(hash uint64) int {
	return int(hash % 22447712886978529)
}
func moduloSize_28282345988300791(hash uint64) int {
	return int(hash % 28282345988300791)
}
func moduloSize_35633523051069991(hash uint64) int {
	return int(hash % 35633523051069991)
}

func moduloSize_44895425773957261(hash uint64) int {
	return int(hash % 44895425773957261)
}
func moduloSize_56564691976601587(hash uint64) int {
	return int(hash % 56564691976601587)
}
func moduloSize_71267046102139967(hash uint64) int {
	return int(hash % 71267046102139967)
}

func moduloSize_89790851547914507(hash uint64) int {
	return int(hash % 89790851547914507)
}
func moduloSize_113129383953203213(hash uint64) int {
	return int(hash % 113129383953203213)
}
func moduloSize_142534092204280003(hash uint64) int {
	return int(hash % 142534092204280003)
}

func moduloSize_179581703095829107(hash uint64) int {
	return int(hash % 179581703095829107)
}
func moduloSize_226258767906406483(hash uint64) int {
	return int(hash % 226258767906406483)
}
func moduloSize_285068184408560057(hash uint64) int {
	return int(hash % 285068184408560057)
}

func moduloSize_359163406191658253(hash uint64) int {
	return int(hash % 359163406191658253)
}
func moduloSize_452517535812813007(hash uint64) int {
	return int(hash % 452517535812813007)
}
func moduloSize_570136368817120201(hash uint64) int {
	return int(hash % 570136368817120201)
}

func moduloSize_718326812383316683(hash uint64) int {
	return int(hash % 718326812383316683)
}
func moduloSize_905035071625626043(hash uint64) int {
	return int(hash % 905035071625626043)
}
func moduloSize_1140272737634240411(hash uint64) int {
	return int(hash % 1140272737634240411)
}

func moduloSize_1436653624766633509(hash uint64) int {
	return int(hash % 1436653624766633509)
}
func moduloSize_1810070143251252131(hash uint64) int {
	return int(hash % 1810070143251252131)
}
func moduloSize_2280545475268481167(hash uint64) int {
	return int(hash % 2280545475268481167)
}

func moduloSize_2873307249533267101(hash uint64) int {
	return int(hash % 2873307249533267101)
}
func moduloSize_3620140286502504283(hash uint64) int {
	return int(hash % 3620140286502504283)
}
func moduloSize_4561090950536962147(hash uint64) int {
	return int(hash % 4561090950536962147)
}

func moduloSize_5746614499066534157(hash uint64) int {
	return int(hash % 5746614499066534157)
}
func moduloSize_7240280573005008577(hash uint64) int {
	return int(hash % 7240280573005008577)
}

func moduloSize_9122181901073924329(hash uint64) int {
	return int(hash % 9122181901073924329)
}
