# KeyIP-Intelligence API Reference

> Version: v1 | Base URL: `https://api.keyip-intelligence.com/api/v1`

---

## Authentication

All API requests require a Bearer token obtained via OAuth 2.0 / OIDC from the Keycloak identity provider.

````

Authorization: Bearer <access_token>

````

### Obtaining a Token

```bash
curl -X POST https://auth.keyip-intelligence.com/realms/keyip/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=your-client-id" \
  -d "client_secret=your-client-secret"
````

---

## Common Response Format

All responses follow a consistent envelope:

```json
{
  "code": 0,
  "message": "success",
  "data": { ... },
  "meta": {
    "request_id": "req_abc123",
    "timestamp": "2025-01-15T10:30:00Z",
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total": 150,
      "total_pages": 8
    }
  }
}
```

### Error Response

```json
{
  "code": 40001,
  "message": "Invalid SMILES string: unclosed ring at position 15",
  "data": null,
  "meta": {
    "request_id": "req_def456",
    "timestamp": "2025-01-15T10:30:01Z"
  }
}
```

### Error Codes

| Code Range  | Category              | Examples                                  |
| :---------- | :-------------------- | :---------------------------------------- |
| 0           | Success               | —                                         |
| 40001-40099 | Validation errors     | Invalid SMILES, missing required field    |
| 40100-40199 | Authentication errors | Invalid token, expired token              |
| 40300-40399 | Authorization errors  | Insufficient permissions, tenant mismatch |
| 40400-40499 | Not found             | Patent not found, molecule not found      |
| 42900-42999 | Rate limiting         | Too many requests                         |
| 50000-50099 | Internal errors       | Database error, model inference failure   |
| 50300-50399 | Service unavailable   | Model loading, maintenance mode           |

---

## Molecule APIs

### POST /molecules/similarity-search

Search for patents containing molecules structurally similar to the input.

**Request Body:**

```json
{
  "molecule": {
    "format": "smiles",
    "value": "c1ccc2c(c1)c1ccccc1n2-c1ccccc1"
  },
  "search_params": {
    "similarity_metric": "tanimoto",
    "similarity_threshold": 0.70,
    "fingerprint_type": "morgan",
    "max_results": 20,
    "patent_offices": ["CNIPA", "USPTO", "EPO"],
    "date_range": {
      "from": "2020-01-01",
      "to": "2025-01-01"
    },
    "oled_roles": ["emitter", "host"]
  },
  "analysis_options": {
    "include_claim_analysis": true,
    "include_infringement_risk": true,
    "include_property_comparison": false
  }
}
```

| Field                                          | Type     | Required | Description                                             |
| :--------------------------------------------- | :------- | :------- | :------------------------------------------------------ |
| `molecule.format`                              | string   | Yes      | `smiles`, `inchi`, `mol` (MDL Molfile)                  |
| `molecule.value`                               | string   | Yes      | Molecular structure string                              |
| `search_params.similarity_metric`              | string   | No       | `tanimoto` (default), `dice`, `cosine`, `gnn_embedding` |
| `search_params.similarity_threshold`           | float    | No       | 0.0-1.0, default 0.70                                   |
| `search_params.fingerprint_type`               | string   | No       | `morgan` (default), `maccs`, `topological`              |
| `search_params.max_results`                    | int      | No       | 1-100, default 20                                       |
| `search_params.patent_offices`                 | string[] | No       | Filter by patent office                                 |
| `search_params.date_range`                     | object   | No       | Filter by filing/publication date                       |
| `search_params.oled_roles`                     | string[] | No       | Filter by OLED material role                            |
| `analysis_options.include_claim_analysis`      | bool     | No       | Include claim coverage analysis                         |
| `analysis_options.include_infringement_risk`   | bool     | No       | Include infringement risk score                         |
| `analysis_options.include_property_comparison` | bool     | No       | Include material property comparison                    |

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "query_molecule": {
      "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccccc1",
      "canonical_smiles": "c1ccc(-n2c3ccccc3c3ccccc32)cc1",
      "molecular_formula": "C18H13N",
      "molecular_weight": 255.31,
      "inchi_key": "DMBHHRLKUKUOEG-UHFFFAOYSA-N"
    },
    "hits": [
      {
        "patent_number": "CN115000001A",
        "title": "一种含有咔唑基团的有机发光材料及其应用",
        "assignee": "某某材料科技有限公司",
        "filing_date": "2022-06-15",
        "publication_date": "2022-09-20",
        "jurisdiction": "CN",
        "legal_status": "granted",
        "matched_molecule": {
          "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccc(C)cc1",
          "similarity_score": 0.92,
          "similarity_metric": "tanimoto",
          "structural_difference": "methyl substituent at para position"
        },
        "claim_analysis": {
          "covering_claims": [1, 3, 7],
          "claim_1_text": "一种有机发光化合物，其特征在于具有通式(I)所示结构...",
          "coverage_type": "markush_specific",
          "markush_match": true
        },
        "infringement_risk": {
          "overall_score": 0.85,
          "risk_level": "high",
          "literal_infringement": 0.78,
          "doctrine_of_equivalents": 0.91,
          "key_factors": [
            "Core carbazole scaffold identical",
            "Substituent falls within Markush definition",
            "Functional equivalence in OLED device context"
          ]
        }
      }
    ],
    "search_metadata": {
      "total_hits": 47,
      "returned_hits": 20,
      "search_time_ms": 342,
      "corpus_size": 128500,
      "fingerprint_type": "morgan",
      "similarity_metric": "tanimoto"
    }
  },
  "meta": {
    "request_id": "req_mol_001",
    "timestamp": "2025-01-15T10:30:00Z",
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total": 47,
      "total_pages": 3
    }
  }
}
```

---

### POST /molecules/substructure-search

Search for patents containing molecules with a specific substructure.

**Request Body:**

```json
{
  "substructure": {
    "format": "smarts",
    "value": "[#6]1:[#6]:[#6]:[#6]2:[#6](:[#6]:1):[#6]1:[#6]:[#6]:[#6]:[#6]:[#6]:1:[#7]:2"
  },
  "search_params": {
    "max_results": 50,
    "patent_offices": ["CNIPA", "USPTO"]
  }
}
```

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "substructure_smarts": "[#6]1:[#6]:[#6]:[#6]2:[#6](:[#6]:1):[#6]1:[#6]:[#6]:[#6]:[#6]:[#6]:1:[#7]:2",
    "substructure_name": "carbazole",
    "hits": [
      {
        "patent_number": "US11234567B2",
        "matched_molecules": [
          {
            "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccc(-c2ccccc2)cc1",
            "match_positions": [[0, 12]],
            "oled_role": "host"
          }
        ]
      }
    ],
    "total_hits": 1250
  }
}
```

---

### GET /molecules/{id}

Retrieve detailed information about a specific molecule.

**Path Parameters:**

| Parameter | Type | Description         |
| :-------- | :--- | :------------------ |
| `id`      | UUID | Molecule identifier |

**Query Parameters:**

| Parameter            | Type | Default | Description                      |
| :------------------- | :--- | :------ | :------------------------------- |
| `include_patents`    | bool | false   | Include associated patents       |
| `include_properties` | bool | false   | Include material properties      |
| `include_similar`    | bool | false   | Include top-10 similar molecules |

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccccc1",
    "canonical_smiles": "c1ccc(-n2c3ccccc3c3ccccc32)cc1",
    "inchi": "InChI=1S/C18H13N/c1-3-7-13(8-4-1)19-17-11-5-2-6-12(17)16-15(19)10-9-14(16)18-13/h1-13H",
    "inchi_key": "DMBHHRLKUKUOEG-UHFFFAOYSA-N",
    "molecular_formula": "C18H13N",
    "molecular_weight": 255.31,
    "oled_role": "host",
    "associated_patents_count": 23,
    "first_disclosed_in": "CN100000001A",
    "first_disclosure_date": "2005-03-15",
    "created_at": "2024-06-01T00:00:00Z",
    "updated_at": "2025-01-10T12:00:00Z"
  }
}
```

---

## Patent APIs

### GET /patents/{patent_number}

Retrieve detailed patent information.

**Path Parameters:**

| Parameter       | Type   | Description                                      |
| :-------------- | :----- | :----------------------------------------------- |
| `patent_number` | string | Patent publication number (e.g., `CN115000001A`) |

**Query Parameters:**

| Parameter           | Type | Default | Description                 |
| :------------------ | :--- | :------ | :-------------------------- |
| `include_claims`    | bool | false   | Include parsed claims       |
| `include_molecules` | bool | false   | Include extracted molecules |
| `include_family`    | bool | false   | Include patent family       |
| `include_citations` | bool | false   | Include citation network    |

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "patent_number": "CN115000001A",
    "title": "一种含有咔唑基团的有机发光材料及其应用",
    "abstract": "本发明公开了一种含有咔唑基团的有机发光材料...",
    "filing_date": "2022-06-15",
    "publication_date": "2022-09-20",
    "grant_date": "2023-12-01",
    "legal_status": "granted",
    "jurisdiction": "CN",
    "assignee": {
      "name": "某某材料科技有限公司",
      "country": "CN"
    },
    "inventors": [
      { "name": "张某某", "affiliation": "某某材料科技有限公司" }
    ],
    "ipc_codes": ["C07D209/86", "C09K11/06", "H10K85/60"],
    "claims": [
      {
        "claim_number": 1,
        "claim_type": "independent",
        "claim_text": "一种有机发光化合物，其特征在于具有通式(I)所示结构...",
        "elements": [
          {
            "element_id": "1a",
            "text": "有机发光化合物",
            "type": "preamble"
          },
          {
            "element_id": "1b",
            "text": "具有通式(I)所示结构",
            "type": "body",
            "has_markush": true
          }
        ]
      }
    ],
    "extracted_molecules": [
      {
        "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccc(C)cc1",
        "role_in_patent": "example_compound",
        "example_number": 1
      }
    ],
    "family": {
      "family_id": "FAM_2022_001",
      "members": [
        { "patent_number": "CN115000001A", "jurisdiction": "CN", "status": "granted" },
        { "patent_number": "US2023/0100001A1", "jurisdiction": "US", "status": "pending" },
        { "patent_number": "EP4200001A1", "jurisdiction": "EP", "status": "pending" }
      ]
    }
  }
}
```

---

### POST /patents/search

Full-text and structured search across the patent corpus.

**Request Body:**

```json
{
  "query": {
    "text": "carbazole OLED host material",
    "fields": ["title", "abstract", "claims"],
    "language": "en"
  },
  "filters": {
    "jurisdictions": ["CN", "US", "EP"],
    "date_range": {
      "field": "filing_date",
      "from": "2020-01-01",
      "to": "2025-01-01"
    },
    "legal_status": ["granted", "pending"],
    "assignees": [],
    "ipc_codes": ["C07D209/*", "H10K85/*"]
  },
  "sort": {
    "field": "relevance",
    "order": "desc"
  },
  "pagination": {
    "page": 1,
    "page_size": 20
  }
}
```

---

## Infringement APIs

### POST /infringement/assess

Assess infringement risk for a molecule against one or more patents.

**Request Body:**

```json
{
  "target_molecule": {
    "format": "smiles",
    "value": "c1ccc2c(c1)c1ccccc1n2-c1ccc(OC)cc1"
  },
  "patents": ["CN115000001A", "US11234567B2"],
  "assessment_options": {
    "include_literal_infringement": true,
    "include_doctrine_of_equivalents": true,
    "include_prosecution_history": false,
    "claim_types": ["independent"],
    "jurisdictions": ["CN", "US"]
  }
}
```

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "target_molecule": {
      "smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccc(OC)cc1",
      "canonical_smiles": "COc1ccc(-n2c3ccccc3c3ccccc32)cc1"
    },
    "assessments": [
      {
        "patent_number": "CN115000001A",
        "jurisdiction": "CN",
        "overall_risk": {
          "score": 0.78,
          "level": "high",
          "confidence": 0.85
        },
        "literal_infringement": {
          "score": 0.65,
          "claim_element_mapping": [
            {
              "claim_number": 1,
              "element_id": "1a",
              "element_text": "有机发光化合物",
              "matched": true,
              "match_explanation": "Target molecule is an organic light-emitting compound"
            },
            {
              "claim_number": 1,
              "element_id": "1b",
              "element_text": "具有通式(I)所示结构",
              "matched": true,
              "match_explanation": "Carbazole core with aryl substituent falls within Markush definition; methoxy group at para position is encompassed by R1 = C1-C6 alkoxy"
            }
          ],
          "all_elements_matched": true
        },
        "doctrine_of_equivalents": {
          "score": 0.88,
          "analysis": {
            "function_way_result": {
              "same_function": true,
              "same_way": true,
              "same_result": true,
              "explanation": "Methoxy substituent performs substantially the same function (electron donation) in substantially the same way (mesomeric effect through oxygen lone pair) to achieve substantially the same result (hole transport enhancement)"
            }
          }
        },
        "recommendations": [
          "High infringement risk detected for CN115000001A Claim 1",
          "Consider design-around: modify substituent position from para to meta",
          "Consider design-around: replace alkoxy with fluoroalkyl group",
          "Recommend formal FTO opinion from patent counsel"
        ]
      },
      {
        "patent_number": "US11234567B2",
        "jurisdiction": "US",
        "overall_risk": {
          "score": 0.35,
          "level": "low",
          "confidence": 0.72
        },
        "literal_infringement": {
          "score": 0.30,
          "claim_element_mapping": [
            {
              "claim_number": 1,
              "element_id": "1a",
              "matched": true
            },
            {
              "claim_number": 1,
              "element_id": "1b",
              "matched": false,
              "match_explanation": "US patent requires spiro linkage which is absent in target molecule"
            }
          ],
          "all_elements_matched": false
        },
        "doctrine_of_equivalents": {
          "score": 0.40,
          "analysis": {
            "function_way_result": {
              "same_function": true,
              "same_way": false,
              "same_result": true,
              "explanation": "Different structural topology (planar vs spiro) constitutes a substantially different way"
            }
          }
        },
        "recommendations": [
          "Low infringement risk for US11234567B2",
          "Spiro linkage requirement in Claim 1 distinguishes target molecule"
        ]
      }
    ],
    "assessment_metadata": {
      "models_used": ["InfringeNet v1.2", "ClaimBERT v1.0"],
      "assessment_time_ms": 1850,
      "disclaimer": "This assessment is AI-generated and does not constitute legal advice. Consult qualified patent counsel for formal opinions."
    }
  }
}
```

---

## Lifecycle APIs

### GET /lifecycle/deadlines

Retrieve upcoming statutory deadlines across jurisdictions.

**Query Parameters:**

| Parameter        | Type   | Default | Description                                                 |
| :--------------- | :----- | :------ | :---------------------------------------------------------- |
| `days`           | int    | 90      | Look-ahead window in days                                   |
| `jurisdictions`  | string | all     | Comma-separated jurisdiction codes                          |
| `deadline_types` | string | all     | `response`, `annuity`, `pct_entry`, `examination`, `appeal` |
| `portfolio_id`   | UUID   | —       | Filter by portfolio                                         |
| `priority`       | string | all     | `critical`, `high`, `medium`, `low`                         |
| `page`           | int    | 1       | Page number                                                 |
| `page_size`      | int    | 20      | Results per page                                            |

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "deadlines": [
      {
        "id": "dl_001",
        "patent_number": "CN115000001A",
        "jurisdiction": "CN",
        "deadline_type": "annuity",
        "deadline_date": "2025-06-15",
        "days_remaining": 152,
        "priority": "high",
        "description": "第3年年费缴纳截止日",
        "fee": {
          "amount": 900.00,
          "currency": "CNY",
          "late_fee_surcharge": {
            "1_month": 0,
            "2_months": 225,
            "3_months": 450,
            "4_months": 675,
            "5_months": 900,
            "6_months": 1125
          }
        },
        "status": "pending",
        "assignee": "某某材料科技有限公司",
        "responsible_agent": "北京某某专利代理事务所",
        "notes": ""
      },
      {
        "id": "dl_002",
        "patent_number": "US2023/0100001A1",
        "jurisdiction": "US",
        "deadline_type": "response",
        "deadline_date": "2025-03-20",
        "days_remaining": 65,
        "priority": "critical",
        "description": "Office Action response deadline (non-final rejection)",
        "fee": {
          "amount": 0,
          "currency": "USD"
        },
        "extensions_available": {
          "1_month": { "fee": 200, "currency": "USD" },
          "2_months": { "fee": 600, "currency": "USD" },
          "3_months": { "fee": 1400, "currency": "USD" }
        },
        "status": "pending",
        "assignee": "某某材料科技有限公司",
        "responsible_agent": "US Patent Agency LLC"
      }
    ],
    "summary": {
      "total_deadlines": 15,
      "critical": 2,
      "high": 5,
      "medium": 6,
      "low": 2,
      "total_fees_due": {
        "CNY": 4500.00,
        "USD": 3200.00,
        "EUR": 1800.00
      }
    }
  }
}
```

---

### POST /lifecycle/annuities/calculate

Calculate annuity fees for a patent across its remaining lifetime.

**Request Body:**

```json
{
  "patent_number": "CN115000001A",
  "calculation_options": {
    "include_late_fees": true,
    "include_currency_conversion": true,
    "base_currency": "CNY",
    "projection_years": 10
  }
}
```

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "patent_number": "CN115000001A",
    "jurisdiction": "CN",
    "filing_date": "2022-06-15",
    "max_term_date": "2042-06-15",
    "annuity_schedule": [
      { "year": 3, "due_date": "2025-06-15", "fee_cny": 900, "status": "upcoming" },
      { "year": 4, "due_date": "2026-06-15", "fee_cny": 1200, "status": "future" },
      { "year": 5, "due_date": "2027-06-15", "fee_cny": 1500, "status": "future" },
      { "year": 6, "due_date": "2028-06-15", "fee_cny": 2000, "status": "future" },
      { "year": 7, "due_date": "2029-06-15", "fee_cny": 2000, "status": "future" },
      { "year": 8, "due_date": "2030-06-15", "fee_cny": 2000, "status": "future" },
      { "year": 9, "due_date": "2031-06-15", "fee_cny": 4000, "status": "future" },
      { "year": 10, "due_date": "2032-06-15", "fee_cny": 4000, "status": "future" }
    ],
    "total_remaining_fees_cny": 17600,
    "total_remaining_fees_usd": 2420.00,
    "exchange_rate": { "CNY_USD": 0.1375, "rate_date": "2025-01-15" }
  }
}
```

---

## Portfolio APIs

### GET /portfolios/{id}/constellation

Retrieve the patent constellation map data for visualization.

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "portfolio_id": "port_001",
    "portfolio_name": "OLED Host Materials Portfolio",
    "visualization": {
      "projection_method": "t-SNE",
      "dimensions": 2,
      "points": [
        {
          "patent_number": "CN115000001A",
          "x": 12.45,
          "y": -3.21,
          "cluster_id": 1,
          "cluster_label": "Carbazole-based hosts",
          "owner": "self",
          "value_score": 8.5,
          "risk_score": 2.1
        },
        {
          "patent_number": "CN116000002B",
          "x": 14.02,
          "y": -2.88,
          "cluster_id": 1,
          "cluster_label": "Carbazole-based hosts",
          "owner": "self",
          "value_score": 7.2,
          "risk_score": 1.5
        },
        {
          "patent_number": "KR102500001B1",
          "x": 13.10,
          "y": -3.50,
          "cluster_id": 1,
          "cluster_label": "Carbazole-based hosts",
          "owner": "competitor_A",
          "value_score": null,
          "risk_score": 6.8
        }
      ],
      "clusters": [
        {
          "cluster_id": 1,
          "label": "Carbazole-based hosts",
          "center_x": 13.19,
          "center_y": -3.20,
          "patent_count": 45,
          "self_count": 12,
          "competitor_count": 33
        }
      ],
      "gaps": [
        {
          "gap_id": "gap_001",
          "center_x": 8.50,
          "center_y": 5.20,
          "radius": 2.5,
          "description": "Triazine-carbazole hybrid hosts — low patent density",
          "opportunity_score": 7.8
        }
      ]
    }
  }
}
```

---

### POST /portfolios/{id}/valuation

Trigger a multi-dimensional valuation of the portfolio.

**Request Body:**

```json
{
  "valuation_dimensions": ["technical", "legal", "commercial", "strategic"],
  "benchmark_competitors": ["competitor_A", "competitor_B"],
  "include_recommendations": true
}
```

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "portfolio_id": "port_001",
    "valuation_date": "2025-01-15",
    "overall_score": 7.4,
    "dimensions": {
      "technical": {
        "score": 8.1,
        "factors": {
          "novelty_breadth": 8.5,
          "claim_strength": 7.8,
          "enablement_quality": 8.0
        }
      },
      "legal": {
        "score": 7.2,
        "factors": {
          "prosecution_robustness": 7.5,
          "validity_risk": 6.8,
          "enforceability": 7.3
        }
      },
      "commercial": {
        "score": 6.8,
        "factors": {
          "market_relevance": 7.5,
          "licensing_potential": 6.0,
          "blocking_power": 6.9
        }
      },
      "strategic": {
        "score": 7.5,
        "factors": {
          "competitive_positioning": 7.8,
          "white_space_coverage": 7.0,
          "defensive_strength": 7.7
        }
      }
    },
    "recommendations": [
      {
        "type": "reinforce",
        "priority": "high",
        "description": "File continuation applications for CN115000001A to broaden Markush coverage in triazine-carbazole hybrid space (gap_001)",
        "estimated_cost": { "amount": 50000, "currency": "CNY" },
        "estimated_value_increase": 0.8
      },
      {
        "type": "abandon",
        "priority": "medium",
        "description": "Consider abandoning CN112000005A — low commercial relevance (score 3.2), annual maintenance cost exceeds strategic value",
        "estimated_savings": { "amount": 4000, "currency": "CNY", "per_year": true }
      }
    ]
  }
}
```

---

## Collaboration APIs

### POST /workspaces

Create a new collaboration workspace.

**Request Body:**

```json
{
  "name": "FTO Analysis — Project Alpha",
  "description": "Freedom-to-operate analysis for new blue emitter candidate",
  "type": "fto_analysis",
  "members": [
    { "user_id": "user_001", "role": "owner" },
    { "user_id": "user_002", "role": "contributor" },
    { "email": "agent@patent-agency.com", "role": "external_reviewer" }
  ],
  "settings": {
    "watermark_enabled": true,
    "download_restricted": true,
    "expiry_date": "2025-06-30"
  }
}
```

**Response (201 Created):**

```json
{
  "code": 0,
  "message": "workspace created",
  "data": {
    "workspace_id": "ws_alpha_001",
    "name": "FTO Analysis — Project Alpha",
    "created_at": "2025-01-15T10:30:00Z",
    "invite_link": "https://app.keyip-intelligence.com/workspace/invite/abc123",
    "members": [
      { "user_id": "user_001", "role": "owner", "status": "active" },
      { "user_id": "user_002", "role": "contributor", "status": "active" },
      { "email": "agent@patent-agency.com", "role": "external_reviewer", "status": "invited" }
    ]
  }
}
```

---

## Report APIs

### POST /reports/fto

Generate a Freedom-to-Operate report.

**Request Body:**

```json
{
  "target_molecules": [
    { "format": "smiles", "value": "c1ccc2c(c1)c1ccccc1n2-c1ccc(OC)cc1", "label": "Candidate A" },
    { "format": "smiles", "value": "c1ccc2c(c1)c1ccccc1n2-c1ccc(F)cc1", "label": "Candidate B" }
  ],
  "scope": {
    "jurisdictions": ["CN", "US", "EP", "JP", "KR"],
    "date_cutoff": "2025-01-15",
    "include_pending": true,
    "include_expired": false
  },
  "report_options": {
    "format": "pdf",
    "language": "zh-CN",
    "detail_level": "full",
    "include_design_around_suggestions": true,
    "include_claim_charts": true
  }
}
```

**Response (202 Accepted):**

```json
{
  "code": 0,
  "message": "report generation started",
  "data": {
    "report_id": "rpt_fto_001",
    "status": "processing",
    "estimated_completion_minutes": 15,
    "poll_url": "/api/v1/reports/rpt_fto_001/status",
    "download_url": "/api/v1/reports/rpt_fto_001/download"
  }
}
```

### GET /reports/{report_id}/status

Poll report generation status.

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "report_id": "rpt_fto_001",
    "status": "completed",
    "progress_percent": 100,
    "created_at": "2025-01-15T10:30:00Z",
    "completed_at": "2025-01-15T10:42:30Z",
    "download_url": "/api/v1/reports/rpt_fto_001/download",
    "file_size_bytes": 2458000,
    "format": "pdf",
    "pages": 35
  }
}
```

---

## Query APIs

### POST /query/natural-language

Ask questions in natural language against the knowledge graph.

**Request Body:**

```json
{
  "question": "哪些竞争对手在2023年之后申请了含有热活化延迟荧光(TADF)机制的蓝光OLED材料专利？",
  "context": {
    "portfolio_id": "port_001",
    "language": "zh-CN"
  },
  "response_options": {
    "include_sources": true,
    "include_visualization": true,
    "max_results": 10
  }
}
```

**Response (200 OK):**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "answer": "根据知识图谱分析，2023年1月至今共有5家主要竞争对手申请了含TADF机制的蓝光OLED材料专利，共计37件。其中：\n\n1. 竞争对手A（韩国）：14件，主要集中在硼氮杂环TADF发光体\n2. 竞争对手B（日本）：9件，聚焦多重共振(MR)-TADF材料\n3. 竞争对手C（中国）：7件，涵盖咔唑-三嗪型TADF主体材料\n4. 竞争对手D（德国）：4件，铜配合物TADF发光体\n5. 竞争对手E（美国）：3件，TADF敏化荧光体系",
    "sources": [
      { "patent_number": "KR102600001B1", "relevance": 0.95 },
      { "patent_number": "JP2024-001234A", "relevance": 0.92 },
      { "patent_number": "CN117000003A", "relevance": 0.88 }
    ],
    "visualization": {
      "type": "bar_chart",
      "data": {
        "labels": ["竞争对手A", "竞争对手B", "竞争对手C", "竞争对手D", "竞争对手E"],
        "values": [14, 9, 7, 4, 3]
      }
    },
    "confidence": 0.88,
    "kg_query_generated": "MATCH (p:Patent)-[:ASSIGNED_TO]->(a:Assignee) WHERE p.filing_date >= '2023-01-01' AND p.abstract CONTAINS 'TADF' AND p.ipc_codes CONTAINS 'H10K85' RETURN a.name, count(p) ORDER BY count(p) DESC"
  }
}
```

---

## Rate Limits

| Tier             | Requests/min | Similarity searches/min | Report generations/hour |
| :--------------- | :----------- | :---------------------- | :---------------------- |
| **Free**         | 60           | 10                      | 2                       |
| **Professional** | 300          | 50                      | 20                      |
| **Enterprise**   | 1000         | 200                     | Unlimited               |

Rate limit headers are included in every response:

```
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 287
X-RateLimit-Reset: 1705312260
```

---

## Webhooks

Subscribe to events for real-time notifications.

### Supported Events

| Event                   | Description                          |
| :---------------------- | :----------------------------------- |
| `patent.new_match`      | New patent matches a monitoring rule |
| `patent.status_changed` | Legal status change detected         |
| `deadline.approaching`  | Deadline within configured threshold |
| `deadline.overdue`      | Deadline has passed without action   |
| `report.completed`      | Report generation completed          |
| `infringement.alert`    | High-risk infringement detected      |

### Webhook Payload

```json
{
  "event": "patent.new_match",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "monitoring_rule_id": "rule_001",
    "patent_number": "CN118000001A",
    "similarity_score": 0.87,
    "matched_molecule_smiles": "c1ccc2c(c1)c1ccccc1n2-c1ccc(N(C)C)cc1"
  },
  "signature": "sha256=abc123..."
}
```
