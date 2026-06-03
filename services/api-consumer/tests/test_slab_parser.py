from services.slab_parser import parse_slab


def test_parse_psa_10():
    result = parse_slab("Radiant Charizard Pokemon GO PSA 10 GEM MT")
    assert result["is_slab"] is True
    assert result["grader"] == "PSA"
    assert result["grade"] == "10"
    assert result["slab_tier"] == "PSA_10"


def test_parse_cgc_decimal_grade():
    result = parse_slab("Charizard ex CGC 9.5 Mint")
    assert result["slab_tier"] == "CGC_9_5"


def test_parse_bgs_black_label():
    result = parse_slab("Pikachu BGS 10 Black Label")
    assert result["slab_tier"] == "BGS_BLACK_LABEL"


def test_raw_card_defaults_to_raw():
    result = parse_slab("Radiant Charizard holo Pokemon GO 011/078")
    assert result["is_slab"] is False
    assert result["slab_tier"] == "RAW"


def test_unknown_graded_listing():
    result = parse_slab("Charizard graded slabbed card")
    assert result["is_slab"] is True
    assert result["slab_tier"] == "GRADED_UNKNOWN"


def test_parse_lower_psa_grade():
    result = parse_slab("2004 Pokemon EX Team Rocket Returns Azumarill PSA 3 VG")
    assert result["is_slab"] is True
    assert result["slab_tier"] == "PSA_3"


def test_parse_gsg_unknown_grade_as_slab():
    result = parse_slab("GSG Marill Azumarill EX Team Rocket Returns English Holo Like PSA BGS")
    assert result["is_slab"] is True
    assert result["grader"] == "GSG"
    assert result["slab_tier"] == "GSG_UNKNOWN"
