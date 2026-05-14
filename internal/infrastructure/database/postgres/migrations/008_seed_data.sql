-- +migrate Up
-- ============================================================================
-- KeyIP-Intelligence: Comprehensive Seed Data
-- Generated: 2026-05-12
-- Covers: organizations, users, molecules, properties, patents, claims, 
--         portfolios, valuations, deadlines, lifecycle, health scores
-- ============================================================================

-- ── Organizations ─────────────────────────────────────────────────────────────
INSERT INTO organizations (id, name, slug, description, plan, max_members, max_patents, settings) VALUES
('a0000001-0000-0000-0000-000000000001', 'OLED材料科技有限公司', 'oled-material-tech', 
  'OLED材料研发与专利管理公司，专注于蓝色OLED主体材料和磷光发光材料', 'enterprise', 50, 5000,
  '{"default_jurisdiction":"CN","allowed_jurisdictions":["CN","US","EP","JP","KR"],"branding":{"primary_color":"#1a56db"}}'),
('a0000001-0000-0000-0000-000000000002', 'Luminara Materials Inc.', 'luminara-materials',
  'US-based OLED emitter materials company with global patent portfolio', 'professional', 20, 1000,
  '{"default_jurisdiction":"US","allowed_jurisdictions":["US","CN","EP","JP","KR"]}');

-- ── Users (all passwords: 123456) ────────────────────────────────────────────
INSERT INTO users (id, email, username, display_name, password_hash, status, avatar_url, locale, timezone, last_login_at, last_login_ip, login_count, email_verified_at, mfa_enabled, preferences, metadata) VALUES
('b0000001-0000-0000-0000-000000000001', 'turta@keyip.io', 'turta', 'Turta',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'zh-CN', 'Asia/Shanghai', NOW(), '192.168.99.1', 1, NOW(), FALSE,
  '{"theme":"dark","notification":"email","dashboard_layout":"analyst"}',
  '{"department":"Executive","title":"CTO","role":"admin","permission_group":"super_admin"}'),

('b0000001-0000-0000-0000-000000000002', 'zhang.wei@oledmaterial.cn', 'zhangwei', '张伟',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'zh-CN', 'Asia/Shanghai', NOW() - interval '1 day', '192.168.1.100', 342, NOW(),
  TRUE, '{"theme":"dark","notification":"email+slack","dashboard_layout":"analyst"}',
  '{"department":"IP Management","title":"IP Manager","role":"admin","permission_group":"super_admin"}'),

('b0000001-0000-0000-0000-000000000003', 'li.jing@research.oled.cn', 'lijing', '李静',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'zh-CN', 'Asia/Shanghai', NOW() - interval '2 days', '10.0.1.50', 189, NOW(),
  FALSE, '{"theme":"light","notification":"email","dashboard_layout":"researcher"}',
  '{"department":"R&D","title":"Senior Research Scientist","role":"researcher","specialization":"TADF materials design"}'),

('b0000001-0000-0000-0000-000000000004', 'chen.yu@oledmaterial.cn', 'chenyu', '陈宇',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'zh-CN', 'Asia/Shanghai', NOW() - interval '6 hours', '192.168.1.200', 521, NOW(),
  TRUE, '{"theme":"dark","notification":"email+slack+wechat","dashboard_layout":"executive"}',
  '{"department":"Executive","title":"VP of IP Strategy","role":"executive","permission_group":"executive"}'),

('b0000001-0000-0000-0000-000000000005', 'wang.fang@ip-agency.cn', 'wangfang', '王芳',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'zh-CN', 'Asia/Shanghai', NOW() - interval '1 day', '203.0.113.45', 87, NOW(),
  TRUE, '{"theme":"light","notification":"email","dashboard_layout":"ip_manager"}',
  '{"department":"Patent Agency","title":"Patent Agent","role":"ip_manager","specialization":"OLED patent prosecution"}'),

('b0000001-0000-0000-0000-000000000006', 'partner@samsung-display.co.kr', 'partner_kim', 'Kim Min-Soo',
  '$2a$10$pQdyk0IfyHMm7g9ZRRsIaOOmeEtUtbJPvFntSF7308CAV0Ht7gVxC', 'active',
  NULL, 'ko-KR', 'Asia/Seoul', NOW() - interval '3 days', '123.45.67.89', 34, NOW(),
  TRUE, '{"theme":"dark","notification":"email","dashboard_layout":"partner"}',
  '{"department":"Partnership","title":"External Collaborator","organization":"Samsung Display","role":"partner_agent","access_scope":"portfolio_share_only"}');

-- ── Org Memberships ──────────────────────────────────────────────────────────
INSERT INTO organization_members (organization_id, user_id, role) VALUES
('a0000001-0000-0000-0000-000000000001', 'b0000001-0000-0000-0000-000000000001', 'owner'),
('a0000001-0000-0000-0000-000000000001', 'b0000001-0000-0000-0000-000000000002', 'admin'),
('a0000001-0000-0000-0000-000000000001', 'b0000001-0000-0000-0000-000000000003', 'member'),
('a0000001-0000-0000-0000-000000000001', 'b0000001-0000-0000-0000-000000000004', 'admin'),
('a0000001-0000-0000-0000-000000000001', 'b0000001-0000-0000-0000-000000000005', 'member'),
('a0000002-0000-0000-0000-000000000002', 'b0000001-0000-0000-0000-000000000006', 'member');

-- ── User Roles (link back to migration 005 system roles) ─────────────────────
INSERT INTO user_roles (user_id, role_id, organization_id) 
SELECT 'b0000001-0000-0000-0000-000000000001', id, 'a0000001-0000-0000-0000-000000000001' FROM roles WHERE name = 'super_admin'
UNION ALL SELECT 'b0000001-0000-0000-0000-000000000002', id, 'a0000001-0000-0000-0000-000000000001' FROM roles WHERE name = 'org_admin'
UNION ALL SELECT 'b0000001-0000-0000-0000-000000000003', id, 'a0000001-0000-0000-0000-000000000001' FROM roles WHERE name = 'researcher'
UNION ALL SELECT 'b0000001-0000-0000-0000-000000000004', id, 'a0000001-0000-0000-0000-000000000001' FROM roles WHERE name = 'patent_analyst'
UNION ALL SELECT 'b0000001-0000-0000-0000-000000000005', id, 'a0000001-0000-0000-0000-000000000001' FROM roles WHERE name = 'patent_analyst'
UNION ALL SELECT 'b0000001-0000-0000-0000-000000000006', id, 'a0000002-0000-0000-0000-000000000002' FROM roles WHERE name = 'viewer';

-- ══════════════════════════════════════════════════════════════════════════════
-- MOLECULES (15 OLED compounds with real structures)
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO molecules (id, smiles, canonical_smiles, inchi, inchi_key, molecular_formula, molecular_weight, exact_mass, logp, tpsa, num_atoms, num_bonds, num_rings, num_aromatic_rings, num_rotatable_bonds, status, name, aliases, source, source_reference, metadata) VALUES
('c0000001-0000-0000-0000-000000000001',
 'c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1',
 'c1ccc(cc1)-c1ccc(cc1)n1c2ccccc2c2ccccc21',
 'InChI=1S/C24H17N/c1-5-13-20(14-6-1)18-9-11-21(12-10-18)25-23-17-8-4-3-7-16(17)22-15-2-19(25)24(22)23/h1-15H',
 'IYZMXHQDXZKNCY-UHFFFAOYSA-N', 'C24H17N', 319.40, 319.1361, 5.89, 6.48, 42, 48, 5, 5, 2,
 'active', 'CBP', ARRAY['4,4''-Bis(9H-carbazol-9-yl)biphenyl','4,4''-Di(9H-carbazol-9-yl)biphenyl'],
 'patent', 'US11678901B2', '{"role":"host","emission_color":"blue","triplet_energy":2.56,"Tg":62,"cas_number":"58328-31-7"}'),

('c0000001-0000-0000-0000-000000000002',
 'c1ccc2c(c1)n(-c1cccc(-n3c4ccccc4c4ccccc43)c1)c2ccccc2',
 'c1ccc2c(c1)n(-c1cccc(c1)n1c3ccccc3c3ccccc31)c2ccccc2',
 'InChI=1S/C24H16N2/c1-3-7-21-15(5-1)17-23(24-19-11-2-4-12-20(19)24)16-18-22(21)25-26(23)13-8-6-9-14-22/h1-14H',
 'VFUDMQLBKNMONU-UHFFFAOYSA-N', 'C24H16N2', 332.40, 332.1313, 5.67, 6.48, 42, 48, 5, 5, 2,
 'active', 'mCP', ARRAY['1,3-Bis(9H-carbazol-9-yl)benzene','mCBP'],
 'patent', 'US11678901B2', '{"role":"host","emission_color":"deep_blue","triplet_energy":2.80,"Tg":93,"cas_number":"342638-54-4"}'),

('c0000001-0000-0000-0000-000000000003',
 'c1ccc(-c2c3ccccc3n(-c3ccc(cc3)n3c4ccccc4c4ccccc43)c3ccccc23)cc1',
 'c1ccc(cc1)-c1cccc2c1c1ccccc1n2-c1ccc(cc1)n1c2ccccc2c2ccccc21',
 'InChI=1S/C36H24N2/c1-3-9-25(10-4-1)29-15-13-23-17-18-24-16-14-30(36-31(29)23)32-26-11-5-2-6-12-27(26)33(32)34-28-19-7-3-8-20-28/h1-20H',
 'GUDOKDNQCQKCOZ-UHFFFAOYSA-N', 'C36H24N2', 484.59, 484.1939, 7.45, 6.48, 62, 72, 7, 7, 3,
 'active', 'BCzPh', ARRAY['9-Phenyl-9H-carbazole-3-ylboronic ester precursor'],
 'patent', 'CN115650927B', '{"role":"host","emission_color":"blue","triplet_energy":2.70,"Tg":78}'),

('c0000001-0000-0000-0000-000000000004',
 'c1ccc(-n(c2ccccc2)c2ccc(cc2)N(c2ccccc2)c2ccccc2)cc1',
 'c1ccc(cc1)N(c1ccccc1)c1ccc(cc1)N(c1ccccc1)c1ccccc1',
 'InChI=1S/C36H28N2/c1-5-13-27(14-6-1)35(28-15-7-2-8-16-28)37-31-21-23-33(24-22-31)38(32-17-9-3-10-18-32)34-25-19-29(20-26-34)36(30-11-4-12-30)39/h1-26H',
 'GXGAKHNRMVGRPK-UHFFFAOYSA-N', 'C36H28N2', 488.62, 488.2252, 8.12, 6.48, 66, 76, 7, 7, 5,
 'active', 'NPB', ARRAY['N,N''-Di(1-naphthyl)-N,N''-diphenylbenzidine','NPD','α-NPD'],
 'patent', 'KR1020230078901B1', '{"role":"HTL","hole_mobility":8.0e-4,"Tg":98,"emission_color":"none"}'),

('c0000001-0000-0000-0000-000000000005',
 'c1ccc(-c2nc(-c3ccccc3)nc(-n3c4ccccc4c4ccccc43)n2)cc1',
 'c1ccc(cc1)-c1nc(nc(-c2ccccc2)n1)-n1c2ccccc2c2ccccc21',
 'InChI=1S/C27H18N4/c1-4-10-20(11-5-1)26-29-27(22-14-8-3-9-15-22)31(30-26)25-23-18-12-6-2-7-13-19(23)24-17-4-3-5-16-24/h1-18H',
 'XGALLCVXZIIBSU-UHFFFAOYSA-N', 'C27H18N4', 398.46, 398.1531, 5.78, 34.15, 49, 57, 6, 6, 3,
 'active', 'TPBi', ARRAY['2,2'',2''''-(1,3,5-Benzinetriyl)-tris(1-phenyl-1H-benzimidazole)'],
 'patent', 'CN114456183B', '{"role":"ETL","electron_mobility":1.0e-5,"Tg":125,"emission_color":"none"}'),

('c0000001-0000-0000-0000-000000000006',
 'c1cnc(-c2ccc(-c3cccc(-c4ccccn4)n3)cc2)c1',
 'c1cnc(cc1)-c1ccc(cc1)-c1cccc(n1)-c1ccccn1',
 'InChI=1S/C20H14N4/c1-2-8-18(21-7-1)20-23-17(13-14-24-20)15-9-11-16(12-10-15)19-22-5-3-4-6-19/h1-14H',
 'HSNGQNXOLWFYIU-UHFFFAOYSA-N', 'C20H14N4', 310.35, 310.1218, 3.45, 52.81, 38, 44, 5, 5, 3,
 'active', 'BPhen', ARRAY['Bathophenanthroline','4,7-Diphenyl-1,10-phenanthroline','Bphen'],
 'patent', 'US11765432B2', '{"role":"ETL","electron_mobility":3.0e-4,"Tg":88,"emission_color":"none"}'),

('c0000001-0000-0000-0000-000000000007',
 'Cc1(c2ccccc2N(c3ccccc13)c1nc(-c2ccccc2)nc(-c2ccccc2)n1)C',
 'CC1(C)c2ccccc2N(c2ccccc12)c1nc(-c2ccccc2)nc(-c2ccccc2)n1',
 'InChI=1S/C30H25N3/c1-30(2)25-18-10-11-19-26(25)33(27-20-14-15-21-27(30)28)29-31-22(23-12-6-3-7-13-23)16-32-24(17-28)29/h3-15,17-21H,1-2H3',
 'WMPVYEBWLRIXDI-UHFFFAOYSA-N', 'C30H25N3', 427.54, 427.2048, 6.78, 16.96, 58, 66, 6, 6, 2,
 'active', 'DMAC-TRZ', ARRAY['9,9-Dimethyl-10-(4,6-diphenyl-1,3,5-triazin-2-yl)-9,10-dihydroacridine'],
 'patent', 'JP2023501234A', '{"role":"emitter","emission_color":"blue","TADF":true,"delta_EST":0.05,"PLQY":0.92,"max_EQE":26.5}'),

('c0000001-0000-0000-0000-000000000008',
 'N#Cc1c(c(c(c(c1n1c2ccccc2c2ccccc21)C#N)n1c2ccccc2c2ccccc21)n1c2ccccc2c2ccccc21)n1c2ccccc2c2ccccc21',
 'N#Cc1c(N#N)c(N#N)c(N#N)c(N#N)c1N#N',
 'InChI=1S/C56H32N6/c1-2-10-38-36(6-1)37-7-3-11-39(38)57(37)54-42(28-27-41(51(54)32-58)52)55-43-29-33-53/h1-28H',
 'VSORRGHJCDEETM-UHFFFAOYSA-N', 'C56H32N6', 788.89, 788.2694, 11.23, 131.16, 94, 108, 14, 14, 4,
 'active', '4CzIPN', ARRAY['2,3,5,6-Tetra(9H-carbazol-9-yl)terephthalonitrile','4CzTPN','4CzIPN-TRZ'],
 'patent', 'JP2023501234A', '{"role":"emitter","emission_color":"green","TADF":true,"delta_EST":0.08,"PLQY":0.94,"max_EQE":19.3}'),

('c0000001-0000-0000-0000-000000000009',
 '[Ir]123(c4ccccc4-c4ccccn41)(c1ccccc1-c1ccccn12)c1ccccc1-c1ccccn13',
 '[Ir]123(c4ccccc4-c4ccccn41)(c1ccccc1-c1ccccn12)c1ccccc1-c1ccccn13',
 'InChI=1S/3C11H8N.Ir/c3*1-2-6-10-9(4-1)5-3-7-12-10;/h3*1-8H;/q3*-1;+3',
 'TZYKPCCYFBBKJQ-UHFFFAOYSA-N', 'C33H24IrN3', 654.78, 655.1687, 6.89, 12.96, 61, 69, 9, 9, 0,
 'active', 'Ir(ppy)3', ARRAY['fac-Tris(2-phenylpyridine)iridium(III)','fac-Ir(ppy)3','Tris(2-phenylpyridinato-C2,N)iridium(III)'],
 'patent', 'US11678901B2', '{"role":"emitter","emission_color":"green","phosphorescent":true,"PLQY":0.97,"emission_wavelength":520,"tau":1.5}'),

('c0000001-0000-0000-0000-000000000010',
 'CC(=O)[O-].[Ir]123(c4ccccc4-c4ccccn41)(c1ccccc1-c1ccccn12)OC(C)=O3',
 'CC(=O)[O-].[Ir](C)(O)(=O)=O',
 'InChI=1S/C22H22N2O4.Ir/c1-15-25-19(20-26-16(2)28-21(19)27-17(3)29-22(20)30-18(4)31)32/h1-4H3;/q;+2/p-2',
 'GJTBVZMKZCMCFK-UHFFFAOYSA-L', 'C22H22IrN2O4', 538.63, 538.1209, 4.23, 52.51, 50, 55, 7, 5, 1,
 'active', 'Ir(ppy)2(acac)', ARRAY['Bis(2-phenylpyridine)(acetylacetonato)iridium(III)','(PPY)2Ir(acac)'],
 'patent', 'US11678901B2', '{"role":"emitter","emission_color":"green","phosphorescent":true,"PLQY":0.80,"emission_wavelength":530}'),

('c0000001-0000-0000-0000-000000000011',
 'c1ccc2c(c1)n1c3ccccc3[Al]3(oc4ccccc4-c4cccn3c4=O)oc1c2=O',
 'c1ccc2c(c1)O[Al]34(Oc5ccccc5C5=CC=C[N]3=C25)Oc1ccccc1C1=CC=C[N]4=C1',
 'InChI=1S/C27H18AlN3O3/c1-4-10-22-19(7-1)25(31-22)32-26-20-8-2-5-11-23(20)27(33-28(26)34)35/h1-15H',
 'AXFQELMGYCLMFL-UHFFFAOYSA-N', 'C27H18AlN3O3', 459.43, 459.1168, 4.56, 52.51, 52, 58, 7, 7, 0,
 'active', 'Alq3', ARRAY['Tris(8-hydroxyquinolinato)aluminum(III)','Tris(8-quinolinolato)aluminum'],
 'patent', 'US11678901B2', '{"role":"ETL_emitter","emission_color":"green","electron_mobility":1.0e-6,"Tg":172}'),

('c0000001-0000-0000-0000-000000000012',
 'c1ccc(-n(c2ccccc2)c2ccc(-c3ccc(-n(c4ccccc4)c4ccccc4)cc3)cc2)cc1',
 'c1ccc(cc1)N(c1ccccc1)c1ccc(cc1)-c1ccc(cc1)N(c1ccccc1)c1ccccc1',
 'InChI=1S/C48H36N2/c1-5-13-39(14-6-1)49(40-15-7-2-8-16-40)41-25-29-45(30-26-41)46-31-27-42(28-32-46)50(43-17-9-3-10-18-43)44-19-11-4-12-20-44/h1-32H',
 'VAYQXVQOCSSLCX-UHFFFAOYSA-N', 'C48H36N2', 640.81, 640.2878, 10.23, 6.48, 86, 100, 9, 9, 7,
 'active', 'TAPC', ARRAY['4,4''-Cyclohexylidenebis[N,N-bis(4-methylphenyl)aniline]','1,1-Bis[(di-4-tolylamino)phenyl]cyclohexane'],
 'patent', 'KR1020230078901B1', '{"role":"HTL","hole_mobility":1.0e-3,"Tg":75,"emission_color":"none"}'),

('c0000001-0000-0000-0000-000000000013',
 'c1ccc(-c2ccc3c(c2)c2ccccc2n3-c2ccccc2)cc1',
 'c1ccc(cc1)-c1ccc2c(c1)c1ccccc1n2-c1ccccc1',
 'InChI=1S/C24H17N/c1-5-13-20(14-6-1)18-9-11-21(12-10-18)25-23-17-8-4-3-7-16(17)22-15-2-19(25)24(22)23/h1-15H',
 'JABPZGMRPMSZPY-UHFFFAOYSA-N', 'C24H17N', 319.40, 319.1361, 5.78, 6.48, 42, 48, 5, 5, 2,
 'active', 'PPZ', ARRAY['10-Phenyl-10H-phenoxazine','Phenoxazine derivative'],
 'synthesis', NULL, '{"role":"donor_building_block","emission_color":"blue","triplet_energy":2.95}'),

('c0000001-0000-0000-0000-000000000014',
 'c1ccc(-n2c3ccccc3c3c4c(ccc32)-c2ccccc2C4(c2ccccc2)c2ccccc2)cc1',
 'c1ccc(cc1)N1c2ccccc2C2(C(=O)c3ccccc3C2c2ccccc2)c2ccccc21',
 'InChI=1S/C30H19NO/c1-3-9-23(10-4-1)31-27-17-7-8-18-28(27)30(24-13-5-2-6-14-24)25-19-11-15-21-29(25)32/h1-19H',
 'QZWMFTWIPROCJH-UHFFFAOYSA-N', 'C30H19NO', 409.48, 409.1467, 6.12, 20.23, 51, 58, 7, 7, 2,
 'active', 'Spiro-CBP', ARRAY['Spiro[fluorene-9,9''-xanthene] carbazole derivative'],
 'synthesis', NULL, '{"role":"host","emission_color":"blue","triplet_energy":2.82,"Tg":145}'),

('c0000001-0000-0000-0000-000000000015',
 'c1ccc(c(cc1)F)n1c2ccc(C(F)(F)F)cc2c2cc(-c3ccc(-c4nc5ccccc5n4)cc3)ccc21',
 'c1ccc(c(c1)F)n1c2ccc(C(F)(F)F)cc2c2cc(ccc21)-c1ccc(-c2nc3ccccc3n2)cc1',
 'InChI=1S/C38H23F4N3/c1-4-10-32(11-5-1)33-22-24-34(26-28(36(24)35(33)41)37(38,39)40)31-18-16-27(17-19-31)30-20-14-25(15-21-30)29-42-45-46/h1-23H',
 'JDKGXWMKQNZFNI-UHFFFAOYSA-N', 'C38H23F4N3', 597.60, 597.1828, 8.45, 16.38, 66, 77, 8, 8, 4,
 'active', 'FIrpic-analog', ARRAY['Iridium(III) bis[2-(4,6-difluorophenyl)pyridinato-N,C2'']picolinate','Flrpic','FIrpic'],
 'patent', 'US11678901B2', '{"role":"emitter","emission_color":"sky_blue","phosphorescent":true,"PLQY":0.78,"emission_wavelength":470}');

-- ── Molecule Properties (selected measurements) ──────────────────────────────
INSERT INTO molecule_properties (molecule_id, property_type, value, unit, measurement_conditions, data_source, confidence) VALUES
('c0000001-0000-0000-0000-000000000001', 'triplet_energy', 2.56, 'eV', '{"temperature":"77K","solvent":"CH2Cl2"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000001', 'homo_level', -6.0, 'eV', '{"method":"UPS"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000001', 'lumo_level', -2.7, 'eV', '{"method":"IPES"}', 'literature', 0.85),
('c0000001-0000-0000-0000-000000000001', 'glass_transition_temp', 62.0, 'C', '{"method":"DSC","ramp":"10°C/min"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000002', 'triplet_energy', 2.80, 'eV', '{"temperature":"77K","solvent":"toluene"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000002', 'homo_level', -6.1, 'eV', '{"method":"UPS"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000002', 'glass_transition_temp', 93.0, 'C', '{"method":"DSC","ramp":"10°C/min"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000007', 'triplet_energy', 2.86, 'eV', '{"temperature":"77K"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000007', 'delta_EST', 0.05, 'eV', '{"method":"measured"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000007', 'PLQY', 92.0, '%', '{"method":"integrating sphere"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000007', 'homo_level', -5.6, 'eV', '{"method":"CV"}', 'literature', 0.90),
('c0000001-0000-0000-0000-000000000008', 'triplet_energy', 2.43, 'eV', '{"temperature":"77K"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000008', 'PLQY', 94.0, '%', '{"method":"integrating sphere","atmosphere":"N2"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000009', 'emission_wavelength', 520.0, 'nm', '{"temperature":"298K","solvent":"CH2Cl2"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000009', 'PLQY', 97.0, '%', '{"method":"integrating sphere"}', 'literature', 0.95),
('c0000001-0000-0000-0000-000000000009', 'triplet_lifetime', 1.5, 'us', '{"temperature":"298K"}', 'literature', 0.95);

-- ══════════════════════════════════════════════════════════════════════════════
-- PATENTS (14 OLED patents across 5 jurisdictions)
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO patents (id, patent_number, title, title_en, abstract, patent_type, status, filing_date, publication_date, grant_date, expiry_date, priority_date, assignee_name, jurisdiction, ipc_codes, family_id, application_number, metadata) VALUES
('d0000001-0000-0000-0000-000000000001', 'CN115650927B', '一种有机发光器件用蓝光主体材料及其制备方法与应用',
 'Blue host material for OLED device, preparation method and application thereof',
 '本发明公开了一种有机发光器件用蓝光主体材料，所述材料具有通式(I)所示结构。核心骨架为咔唑-三嗪双极性结构，三线态能级T1≥2.8eV，玻璃化转变温度Tg≥150℃，适用于蓝色磷光和TADF有机发光器件的主体材料。',
 'invention', 'granted', '2022-03-18', '2023-08-29', '2024-01-05', '2042-03-18', '2022-03-18',
 '深圳光韵达材料科技有限公司', 'CN',
 ARRAY['C07D209/86','C07D403/14','C09K11/06','H10K85/60','H10K50/11'],
 'FAM-2022-OLED-001', 'CN202210271890.5',
 '{"data_source":"CNIPA","technology_field":"OLED_blue_host","commercial_relevance":"high","keyip_tech_codes":["OLED-HOST-BLUE","SPIRO-CARBAZOLE"]}'),

('d0000001-0000-0000-0000-000000000002', 'CN116354946A', '一种有机发光器件用空穴传输材料及其应用',
 'Hole transport material for OLED device and application thereof',
 '本发明涉及一种有机发光器件用空穴传输材料，基于三苯胺-咔唑杂化骨架，HOMO能级至-5.2eV至-5.5eV，空穴迁移率≥1×10⁻³ cm²/Vs，Tg≥130℃。',
 'invention', 'under_examination', '2023-01-10', '2023-07-28', NULL, NULL, '2023-01-10',
 '深圳光韵达材料科技有限公司', 'CN',
 ARRAY['C07D209/86','C07D403/04','H10K85/60','H10K50/15'],
 'FAM-2023-HTL-001', 'CN202310032156.3',
 '{"data_source":"CNIPA","technology_field":"OLED_HTL","commercial_relevance":"medium"}'),

('d0000001-0000-0000-0000-000000000003', 'US11832456B2', 'Phosphorescent Organometallic Compound for Organic Light-Emitting Device',
 'Phosphorescent Organometallic Compound for OLED',
 'Disclosed is a phosphorescent organometallic compound comprising an iridium center coordinated with cyclometalated ligands bearing fluorinated phenylpyridine moieties. The compound exhibits emission in the blue-green region with CIE coordinates of (0.16, 0.32), PLQY exceeding 85%, EQE of 24.5%, and LT95 of 800 hours.',
 'invention', 'granted', '2021-06-15', '2022-11-28', '2023-11-28', '2041-06-15', '2020-12-22',
 'Luminara Materials Inc.', 'US',
 ARRAY['C07F15/00','C09K11/06','H10K85/30','H10K50/11'],
 'FAM-2021-PHO-001', 'US17/349,123',
 '{"data_source":"USPTO","technology_field":"OLED_phosphorescent_emitter","commercial_relevance":"high","priority_country":"KR"}'),

('d0000001-0000-0000-0000-000000000004', 'US20230389421A1', 'Thermally Activated Delayed Fluorescence Material for Blue OLED',
 'TADF Material for Blue OLED',
 'A TADF material based on donor-acceptor architecture with modified acridine donor and diphenyl sulfone acceptor. ΔEST < 0.1 eV, blue emission CIE (0.15, 0.10), PLQY 78%, EQE 20.3% without heavy metal dopant.',
 'invention', 'filed', '2022-08-20', '2023-11-30', NULL, NULL, '2022-02-15',
 'Luminara Materials Inc.', 'US',
 ARRAY['C07D265/38','C07C317/14','C09K11/06','H10K85/60','H10K50/11'],
 'FAM-2022-TADF-001', 'US17/891,456',
 '{"data_source":"USPTO","technology_field":"OLED_TADF_emitter","commercial_relevance":"high","priority_country":"KR"}'),

('d0000001-0000-0000-0000-000000000005', 'EP3985742A1', 'Electron Transport Material for Organic Light-Emitting Device',
 'Electron Transport Material for OLED',
 'Electron transport material based on phosphine oxide-modified triazine scaffold. Electron mobility > 5×10⁻⁴ cm²/Vs, LUMO -2.8 eV, Td > 400°C. Reduced driving voltage by 0.8V vs BPhen, EQE 21%.',
 'invention', 'granted', '2021-09-03', '2022-04-20', '2023-08-16', '2041-09-03', '2021-03-10',
 'European Organic Electronics GmbH', 'EP',
 ARRAY['C07F9/53','C07D251/24','H10K85/60','H10K50/17'],
 'FAM-2021-ETL-001', 'EP21345678.9',
 '{"data_source":"EPO","technology_field":"OLED_ETL","commercial_relevance":"medium"}'),

('d0000001-0000-0000-0000-000000000006', 'JP7234567B2', '有機発光素子およびその製造方法',
 'Organic light-emitting device and manufacturing method thereof',
 'タンデム構造を有する有機発光素子。2倍以上の電流効率、初期輝度5000cd/m²でLT95が1500時間以上。',
 'invention', 'granted', '2021-04-22', '2022-10-15', '2023-03-01', '2041-04-22', '2020-10-30',
 '東京有機エレクトロニクス株式会社', 'JP',
 ARRAY['H10K50/19','H10K50/11','H10K71/00','H10K59/10'],
 'FAM-2021-TANDEM-001', 'JP2021-071234',
 '{"data_source":"JPO","technology_field":"OLED_device_structure","commercial_relevance":"high","priority_country":"JP"}'),

('d0000001-0000-0000-0000-000000000007', 'KR1020230145678A', '유기 발광 소자용 봉지 재료 및 이를 포함하는 유기 발광 표시 장치',
 'Encapsulation material for OLED and display device including the same',
 '다층 박막 봉지 구조. 무기층 Al₂O₃ 또는 SiNx, 유기층 아크릴레이트계 고분자. WVTR ≤ 1×10⁻⁶ g/m²/day, 85℃/85%RH 1000시간 이상 내구성.',
 'invention', 'filed', '2023-04-05', '2023-10-20', NULL, NULL, '2023-04-05',
 '삼성디스플레이 주식회사', 'KR',
 ARRAY['H10K50/84','H10K59/80','C23C16/40','B32B7/02'],
 'FAM-2023-ENCAP-001', 'KR10-2023-0045123',
 '{"data_source":"KIPO","technology_field":"OLED_encapsulation","commercial_relevance":"medium"}'),

('d0000001-0000-0000-0000-000000000008', 'CN108864095B', '一种有机电致发光器件用空穴注入材料',
 'Hole injection material for organic electroluminescent device',
 '基于酞菁铜衍生物的空穴注入材料，HOMO -5.4eV，驱动电压3.2V@10mA/cm²。（已因未缴年费失效）',
 'invention', 'expired', '2017-05-12', '2018-11-23', '2020-02-14', '2037-05-12', '2017-05-12',
 '北京有机光电研究所', 'CN',
 ARRAY['C07D487/22','H10K85/30','H10K50/17'],
 'FAM-2017-HIL-001', 'CN201710334567.8',
 '{"data_source":"CNIPA","technology_field":"OLED_HIL","commercial_relevance":"low","expiry_reason":"failure_to_pay_annuity"}'),

('d0000001-0000-0000-0000-000000000009', 'EP3412156B1', 'Host Material for Phosphorescent Organic Light-Emitting Device',
 'Host Material for Phosphorescent OLED',
 'Bipolar host based on carbazole-benzimidazole hybrid. Hole mobility 2×10⁻⁴ cm²/Vs, electron mobility 8×10⁻⁵ cm²/Vs. Green PhOLED EQE 26%, power efficiency 85 lm/W.（异议撤销）',
 'invention', 'invalidated', '2018-02-08', '2018-12-12', '2020-06-10', NULL, '2017-08-15',
 'European Organic Electronics GmbH', 'EP',
 ARRAY['C07D235/18','C07D209/86','H10K85/60','H10K50/11'],
 'FAM-2018-HOST-001', 'EP18123456.7',
 '{"data_source":"EPO","technology_field":"OLED_host","commercial_relevance":"none","revocation_reason":"opposition_prior_art","revocation_date":"2022-09-20"}'),

('d0000001-0000-0000-0000-000000000010', 'CN116003389B', '一种螺芴-三嗪双极性蓝光主体材料及有机发光器件',
 'Spirofluorene-triazine bipolar blue host material and OLED',
 '螺芴-三嗪骨架双极性蓝光主体材料，T1≥2.85eV，Tg≥165℃。蓝色TADF器件EQE 23.8%，CIE(0.14,0.12)，LT95>600h@1000cd/m²。',
 'invention', 'granted', '2022-06-20', '2023-05-02', '2023-12-15', '2042-06-20', '2022-06-20',
 '深圳光韵达材料科技有限公司', 'CN',
 ARRAY['C07D209/86','C07D251/24','C07D405/14','C09K11/06','H10K85/60'],
 'FAM-2022-OLED-001', 'CN202210700123.4',
 '{"data_source":"CNIPA","technology_field":"OLED_blue_host_bipolar","commercial_relevance":"high","parent_application":"CN115650927B"}'),

('d0000001-0000-0000-0000-000000000011', 'US11456789B2', 'Spirofluorene-Triazine Bipolar Host Material for Blue OLED',
 'Spirofluorene-Triazine Bipolar Host for Blue OLED',
 'A bipolar host material based on spirofluorene-triazine architecture. T1=2.86eV, Tg=168°C. Blue TADF-OLED EQE 23.5%, CIE(0.14,0.13), LT95>580h@1000cd/m².',
 'invention', 'granted', '2022-09-15', '2023-06-20', '2023-10-03', '2042-09-15', '2022-03-18',
 'Luminara Materials Inc.', 'US',
 ARRAY['C07D209/86','C07D251/24','C07D405/14','C09K11/06','H10K85/60'],
 'FAM-2022-OLED-001', 'US17/932,150',
 '{"data_source":"USPTO","technology_field":"OLED_blue_host_bipolar","commercial_relevance":"high","priority_country":"CN"}'),

('d0000001-0000-0000-0000-000000000012', 'EP4123456A1', 'Spirofluorene-Triazine Compound for Organic Electroluminescent Device',
 'Spirofluorene-Triazine for OLED',
 'Donor-acceptor architecture on spirobifluorene scaffold with bipolar charge transport. EQE > 22%, LT95 > 500h@1000cd/m².',
 'invention', 'under_examination', '2022-10-08', '2023-07-12', NULL, NULL, '2022-03-18',
 'Luminara Materials Inc.', 'EP',
 ARRAY['C07D209/86','C07D251/24','H10K85/60'],
 'FAM-2022-OLED-001', 'EP22789123.4',
 '{"data_source":"EPO","technology_field":"OLED_blue_host_bipolar","commercial_relevance":"high","priority_country":"CN"}'),

('d0000001-0000-0000-0000-000000000013', 'CN116425750A', '一种含多重可变基团的有机发光材料及其Markush通式表示',
 'OLED material with multiple variable groups and Markush formula representation thereof',
 '核心骨架为二苯并呋喃/二苯并噻吩与三嗪的杂化结构，Markush通式覆盖超过50000种化合物。T1≥2.7eV，含计算化学筛选方法。',
 'invention', 'under_examination', '2023-03-01', '2023-09-15', NULL, NULL, '2023-03-01',
 '深圳光韵达材料科技有限公司', 'CN',
 ARRAY['C07D307/91','C07D333/76','C07D251/24','C09K11/06','H10K85/60'],
 'FAM-2023-MARKUSH-001', 'CN202310200123.4',
 '{"data_source":"CNIPA","technology_field":"OLED_host_markush","commercial_relevance":"high","markush_complexity":"very_high"}'),

('d0000001-0000-0000-0000-000000000014', 'CN115894555B', '一种有机发光器件的制备方法',
 'Method for preparing organic light-emitting device',
 '采用溶液加工法（空穴层）+真空蒸镀法（发光层）的混合工艺。UV臭氧界面处理+退火消除缺陷。EQE≥20%，制造成本降低40%。',
 'invention', 'granted', '2022-01-15', '2023-04-04', '2023-09-22', '2042-01-15', '2022-01-15',
 '深圳光韵达材料科技有限公司', 'CN',
 ARRAY['H10K71/10','H10K71/12','H10K71/16','H10K50/11'],
 'FAM-2022-METHOD-001', 'CN202210044567.8',
 '{"data_source":"CNIPA","technology_field":"OLED_fabrication_method","commercial_relevance":"medium"}');

-- ── Patent Inventors ─────────────────────────────────────────────────────────
INSERT INTO patent_inventors (patent_id, inventor_name, inventor_name_en, sequence, affiliation) VALUES
('d0000001-0000-0000-0000-000000000001', '张明辉', 'Ming-Hui Zhang', 1, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000001', '李晓燕', 'Xiao-Yan Li', 2, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000001', '王建国', 'Jian-Guo Wang', 3, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000002', '李晓燕', 'Xiao-Yan Li', 1, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000002', '陈伟', 'Wei Chen', 2, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000003', 'Takeshi Yamamoto', 'Takeshi Yamamoto', 1, 'Luminara Materials Inc.'),
('d0000001-0000-0000-0000-000000000003', 'Sarah Chen', 'Sarah Chen', 2, 'Luminara Materials Inc.'),
('d0000001-0000-0000-0000-000000000004', 'Min-Jae Park', 'Min-Jae Park', 1, 'KAIST'),
('d0000001-0000-0000-0000-000000000004', 'Yuki Tanaka', 'Yuki Tanaka', 2, 'Luminara Materials Inc.'),
('d0000001-0000-0000-0000-000000000005', 'Klaus Weber', 'Klaus Weber', 1, 'European Organic Electronics GmbH'),
('d0000001-0000-0000-0000-000000000005', 'Marie Dupont', 'Marie Dupont', 2, 'European Organic Electronics GmbH'),
('d0000001-0000-0000-0000-000000000006', '山本健', 'Takeshi Yamamoto', 1, '東京有機エレクトロニクス株式会社'),
('d0000001-0000-0000-0000-000000000006', '佐藤美咲', 'Misaki Sato', 2, '東京有機エレクトロニクス株式会社'),
('d0000001-0000-0000-0000-000000000007', '김지훈', 'Ji-Hoon Kim', 1, '삼성디스플레이 주식회사'),
('d0000001-0000-0000-0000-000000000007', '이수연', 'Su-Yeon Lee', 2, '삼성디스플레이 주식회사'),
('d0000001-0000-0000-0000-000000000010', '张明辉', 'Ming-Hui Zhang', 1, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000010', '王建国', 'Jian-Guo Wang', 2, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000010', '赵丽华', 'Li-Hua Zhao', 3, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000011', 'Ming-Hui Zhang', 'Ming-Hui Zhang', 1, 'Luminara Materials Inc.'),
('d0000001-0000-0000-0000-000000000011', 'Sarah Chen', 'Sarah Chen', 2, 'Luminara Materials Inc.'),
('d0000001-0000-0000-0000-000000000013', '张明辉', 'Ming-Hui Zhang', 1, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000013', '陈伟', 'Wei Chen', 2, '清华大学'),
('d0000001-0000-0000-0000-000000000014', '王建国', 'Jian-Guo Wang', 1, '深圳光韵达材料科技有限公司'),
('d0000001-0000-0000-0000-000000000014', '赵丽华', 'Li-Hua Zhao', 2, '深圳光韵达材料科技有限公司');

-- ── Patent Claims (representative claims for key patents) ─────────────────────
INSERT INTO patent_claims (id, patent_id, claim_number, claim_type, claim_text, elements) VALUES
('e0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000001', 1, 'independent', 
 '一种有机化合物，具有通式(I)所示结构：其中，Ar1选自苯基、萘基、联苯基、菲基中的一种；Ar2选自吡啶基、嘧啶基、三嗪基中的一种；核心骨架为9,9''-螺二芴与咔唑通过所述L连接的结构。',
 '[{"id":"E1-1","description":"通式(I)所示结构的有机化合物","is_novel":true},{"id":"E1-2","description":"Ar1选自苯基、萘基、联苯基、菲基","is_novel":false},{"id":"E1-3","description":"Ar2选自吡啶基、嘧啶基、三嗪基","is_novel":true},{"id":"E1-6","description":"核心骨架为9,9''-螺二芴与咔唑通过L连接","is_novel":true}]'),
('e0000001-0000-0000-0000-000000000002', 'd0000001-0000-0000-0000-000000000001', 5, 'independent',
 '一种有机发光器件，包括：阳极；空穴注入层；空穴传输层；发光层，所述发光层包含权利要求1至4任一项所述的有机化合物作为主体材料和磷光掺杂剂或TADF掺杂剂；电子传输层；阴极。',
 '[{"id":"E5-1","description":"阳极","is_novel":false},{"id":"E5-4","description":"发光层包含权利要求1-4的化合物作为主体材料","is_novel":true}]'),
('e0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000003', 1, 'independent',
 'An organometallic compound having the formula Ir(L1)₂(L2), wherein L1 is a cyclometalated ligand selected from 2-phenylpyridinato derivatives having at least one fluorine substituent on the phenyl ring, and L2 is an ancillary ligand selected from acetylacetonate, picolinate, and tetrakis(1-pyrazolyl)borate.',
 '[{"id":"E1-1","description":"Organometallic compound with formula Ir(L1)₂(L2)","is_novel":true},{"id":"E1-3","description":"L1 has at least one fluorine on phenyl ring","is_novel":true}]'),
('e0000001-0000-0000-0000-000000000004', 'd0000001-0000-0000-0000-000000000004', 1, 'independent',
 'A thermally activated delayed fluorescence compound comprising a donor moiety and an acceptor moiety connected via a spiro or ortho linkage, wherein the donor moiety is selected from acridine, phenoxazine, and phenothiazine derivatives, and the acceptor moiety is selected from diphenyl sulfone, triazine, and benzonitrile derivatives, and wherein ΔEST < 0.15 eV.',
 '[{"id":"E1-1","description":"TADF compound with donor-acceptor architecture","is_novel":true},{"id":"E1-2","description":"Donor and acceptor connected via spiro or ortho linkage","is_novel":true}]'),
('e0000001-0000-0000-0000-000000000005', 'd0000001-0000-0000-0000-000000000010', 1, 'independent',
 '一种有机化合物，具有通式(III)所示结构，其中核心骨架为9,9''-螺二芴，所述螺二芴的一个芴单元的2位连接给体基团D，7位连接受体基团A；D选自咔唑基、吖啶基、吩噁嗪基中的一种；A选自1,3,5-三嗪基、嘧啶基中的一种。',
 '[{"id":"E1-1","description":"通式(III)所示结构的有机化合物","is_novel":true},{"id":"E1-3","description":"芴2位连接给体D，7位连接受体A","is_novel":true}]'),
('e0000001-0000-0000-0000-000000000006', 'd0000001-0000-0000-0000-000000000013', 1, 'independent',
 '一种有机化合物，具有通式(IV)所示结构，其中X选自O和S；Y选自N和CH；Ar1至Ar3各自独立地选自苯基、萘基、联苯基、菲基、芴基中的一种；R1至R8各自独立地选自氢、C1-C12烷基、C6-C30芳基、C3-C30杂芳基、C6-C30芳氨基中的一种。',
 '[{"id":"E1-1","description":"通式(IV)所示结构的有机化合物","is_novel":true},{"id":"E1-2","description":"X选自O和S（二苯并呋喃或二苯并噻吩）","is_novel":false}]');

-- ── Patent-Molecule Relations ─────────────────────────────────────────────────
INSERT INTO patent_molecule_relations (patent_id, molecule_id, relation_type, location_in_patent, claim_numbers, confidence) VALUES
('d0000001-0000-0000-0000-000000000001', 'c0000001-0000-0000-0000-000000000001', 'prior_art_reference', '背景技术 [0003]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000001', 'c0000001-0000-0000-0000-000000000003', 'disclosed', '实施例1', ARRAY[1,2,3,4,5], 1.0),
('d0000001-0000-0000-0000-000000000001', 'c0000001-0000-0000-0000-000000000014', 'covered_by_claims', '权利要求1实例', ARRAY[1], 0.95),
('d0000001-0000-0000-0000-000000000002', 'c0000001-0000-0000-0000-000000000004', 'prior_art_reference', '背景技术 [0002]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000003', 'c0000001-0000-0000-0000-000000000009', 'prior_art_reference', 'Background [0004]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000003', 'c0000001-0000-0000-0000-000000000010', 'prior_art_reference', 'Background [0005]', NULL, 0.85),
('d0000001-0000-0000-0000-000000000003', 'c0000001-0000-0000-0000-000000000015', 'disclosed', 'Example 1, Compound A', ARRAY[1,2,3,4,5], 1.0),
('d0000001-0000-0000-0000-000000000004', 'c0000001-0000-0000-0000-000000000007', 'covered_by_claims', 'Markush instance', ARRAY[1,2,3,4], 0.95),
('d0000001-0000-0000-0000-000000000004', 'c0000001-0000-0000-0000-000000000008', 'prior_art_reference', 'Background [0003]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000005', 'c0000001-0000-0000-0000-000000000005', 'prior_art_reference', 'Background [0002]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000005', 'c0000001-0000-0000-0000-000000000006', 'comparative_example', 'Comparative Example 1', NULL, 1.0),
('d0000001-0000-0000-0000-000000000006', 'c0000001-0000-0000-0000-000000000011', 'prior_art_reference', '背景技術 [0005]', NULL, 0.8),
('d0000001-0000-0000-0000-000000000010', 'c0000001-0000-0000-0000-000000000001', 'prior_art_reference', '背景技术 [0004]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000010', 'c0000001-0000-0000-0000-000000000003', 'prior_art_reference', '背景技术 [0003]', NULL, 0.9),
('d0000001-0000-0000-0000-000000000010', 'c0000001-0000-0000-0000-000000000014', 'disclosed', '实施例1, 化合物SBH-1', ARRAY[1,2,3,4,5,6], 1.0),
('d0000001-0000-0000-0000-000000000013', 'c0000001-0000-0000-0000-000000000013', 'covered_by_claims', 'Markush 通式实例A', ARRAY[1], 0.95);

-- ══════════════════════════════════════════════════════════════════════════════
-- PORTFOLIOS
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO portfolios (id, name, description, owner_id, status, tech_domains, target_jurisdictions, metadata) VALUES
('f0000001-0000-0000-0000-000000000001', 'Blue Emitter Core Portfolio', 
 'Core defensive portfolio covering blue OLED emitter host materials based on carbazole and spirofluorene scaffolds. Critical for protecting the foundation of the company''s blue OLED technology.',
 'b0000001-0000-0000-0000-000000000001', 'active',
 ARRAY['C09K-011/06','H10K-050/00','H10K-085/60','C07D-209/86'],
 ARRAY['CN','US','EP','JP','KR'],
 '{"strategy":"defensive","risk_level":"low","annual_budget_usd":850000,"review_frequency":"quarterly"}'),

('f0000001-0000-0000-0000-000000000002', 'Hole Transport Material Portfolio',
 'Balanced portfolio for hole transport layer materials including triarylamine and carbazole derivatives. Secondary priority but growing importance for tandem device structures.',
 'b0000001-0000-0000-0000-000000000002', 'active',
 ARRAY['H10K-050/15','H10K-085/00','H10K-085/60','C07C-211/61'],
 ARRAY['CN','US','JP'],
 '{"strategy":"balanced","risk_level":"medium","annual_budget_usd":350000,"review_frequency":"biannual"}'),

('f0000001-0000-0000-0000-000000000003', 'Licensing Revenue Portfolio',
 'High-value patents selected for out-licensing to panel manufacturers. Covers key TADF emitter and device architecture innovations with broad jurisdictional coverage.',
 'b0000001-0000-0000-0000-000000000004', 'active',
 ARRAY['H10K-085/60','H10K-050/11','C09K-011/06','C07F-015/00'],
 ARRAY['US','CN','EP','JP','KR'],
 '{"strategy":"offensive","risk_level":"low","annual_budget_usd":1200000,"revenue_target":"$5M/year"}');

-- ── Portfolio-Patent Assignments ──────────────────────────────────────────────
INSERT INTO portfolio_patents (portfolio_id, patent_id, role_in_portfolio, notes) VALUES
('f0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000001', 'core', 'Foundation patent - blue host material Markush (I)'),
('f0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000010', 'core', 'Divisional with spirofluorene-trizaine architecture'),
('f0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000011', 'core', 'US counterpart of CN116003389B'),
('f0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000012', 'supporting', 'EP counterpart under examination'),
('f0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000013', 'expansion', 'Broad Markush coverage - dibenzofuran variants'),
('f0000001-0000-0000-0000-000000000002', 'd0000001-0000-0000-0000-000000000002', 'core', 'HTL material patent - triarylamine scaffold'),
('f0000001-0000-0000-0000-000000000002', 'd0000001-0000-0000-0000-000000000006', 'supporting', 'Tandem device structure including HTL'),
('f0000001-0000-0000-0000-000000000002', 'd0000001-0000-0000-0000-000000000008', 'legacy', 'Expired HIL patent - historical reference'),
('f0000001-0000-0000-0000-000000000002', 'd0000001-0000-0000-0000-000000000014', 'expansion', 'Solution-process method - enables manufacturing'),
('f0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000003', 'core', 'High-value Ir phosphorescent emitter - US granted'),
('f0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000004', 'core', 'TADF emitter - broad donor/acceptor coverage'),
('f0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000005', 'supporting', 'ETL material - performance vs BPhen benchmark'),
('f0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000007', 'expansion', 'Encapsulation - display integration'),
('f0000001-0000-0000-0000-000000000003', 'd0000001-0000-0000-0000-000000000009', 'legacy', 'Revoked EP patent - IP intelligence reference');

-- ── Patent Valuations ─────────────────────────────────────────────────────────
INSERT INTO patent_valuations (patent_id, portfolio_id, technical_score, legal_score, market_score, strategic_score, composite_score, tier, monetary_value_low, monetary_value_mid, monetary_value_high, currency, valuation_method, scoring_details) VALUES
('d0000001-0000-0000-0000-000000000001', 'f0000001-0000-0000-0000-000000000001', 92, 88, 85, 90, 89.0, 'S', 8000000, 12000000, 18000000, 'USD', 'income_approach',
 '{"technical":{"novelty":95,"feasibility":90,"performance":92},"legal":{"scope_breadth":85,"enforceability":90,"remaining_life":16},"market":{"market_size":85,"growth_rate":90,"competitor_interest":80}}'),
('d0000001-0000-0000-0000-000000000010', 'f0000001-0000-0000-0000-000000000001', 88, 82, 78, 85, 83.5, 'A', 4000000, 7000000, 11000000, 'USD', 'market_approach',
 '{"technical":{"novelty":90,"feasibility":85,"performance":88},"legal":{"scope_breadth":80,"enforceability":82,"remaining_life":16},"market":{"market_size":78,"growth_rate":85,"competitor_interest":72}}'),
('d0000001-0000-0000-0000-000000000011', 'f0000001-0000-0000-0000-000000000001', 90, 90, 88, 88, 89.0, 'S', 6000000, 10000000, 15000000, 'USD', 'income_approach',
 '{"technical":{"novelty":92,"feasibility":88,"performance":90},"legal":{"scope_breadth":88,"enforceability":95,"remaining_life":16},"market":{"market_size":88,"growth_rate":90,"competitor_interest":85}}'),
('d0000001-0000-0000-0000-000000000003', 'f0000001-0000-0000-0000-000000000003', 85, 88, 92, 90, 88.8, 'S', 5000000, 9000000, 14000000, 'USD', 'cost_approach',
 '{"technical":{"novelty":88,"feasibility":82,"performance":85},"legal":{"scope_breadth":86,"enforceability":90,"remaining_life":15},"market":{"market_size":92,"growth_rate":95,"competitor_interest":90}}'),
('d0000001-0000-0000-0000-000000000004', 'f0000001-0000-0000-0000-000000000003', 82, 78, 85, 85, 82.5, 'A', 3000000, 5500000, 9000000, 'USD', 'income_approach',
 '{"technical":{"novelty":85,"feasibility":80,"performance":82},"legal":{"scope_breadth":75,"enforceability":78,"remaining_life":16},"market":{"market_size":85,"growth_rate":90,"competitor_interest":80}}'),
('d0000001-0000-0000-0000-000000000002', 'f0000001-0000-0000-0000-000000000002', 75, 65, 68, 72, 70.0, 'B', 1000000, 2000000, 3500000, 'USD', 'market_approach',
 '{"technical":{"novelty":78,"feasibility":75,"performance":72},"legal":{"scope_breadth":62,"enforceability":65,"remaining_life":17},"market":{"market_size":68,"growth_rate":65,"competitor_interest":70}}'),
('d0000001-0000-0000-0000-000000000005', 'f0000001-0000-0000-0000-000000000003', 78, 75, 72, 74, 74.8, 'B', 1500000, 2800000, 4500000, 'USD', 'income_approach',
 '{"technical":{"novelty":80,"feasibility":78,"performance":76},"legal":{"scope_breadth":72,"enforceability":78,"remaining_life":15},"market":{"market_size":72,"growth_rate":70,"competitor_interest":68}}'),
('d0000001-0000-0000-0000-000000000008', 'f0000001-0000-0000-0000-000000000002', 45, 15, 20, 10, 22.5, 'D', 0, 0, 50000, 'USD', 'cost_approach',
 '{"technical":{"novelty":50,"feasibility":45,"performance":40},"legal":{"scope_breadth":20,"enforceability":0,"remaining_life":0},"market":{"market_size":20,"growth_rate":10,"competitor_interest":5}}'),
('d0000001-0000-0000-0000-000000000009', 'f0000001-0000-0000-0000-000000000003', 70, 10, 25, 15, 30.0, 'D', 0, 0, 100000, 'USD', 'cost_approach',
 '{"technical":{"novelty":72,"feasibility":68,"performance":70},"legal":{"scope_breadth":15,"enforceability":0,"remaining_life":0},"market":{"market_size":25,"growth_rate":15,"competitor_interest":20}}');

-- ── Portfolio Health Scores ───────────────────────────────────────────────────
INSERT INTO portfolio_health_scores (portfolio_id, overall_score, coverage_score, diversity_score, freshness_score, strength_score, risk_score, total_patents, active_patents, expiring_within_year, expiring_within_3years, jurisdiction_distribution, tech_domain_distribution, tier_distribution, recommendations, evaluated_at) VALUES
('f0000001-0000-0000-0000-000000000001', 86.5, 88, 82, 75, 90, 18, 5, 4, 0, 0,
 '{"CN":2,"US":1,"EP":2}',
 '{"OLED_blue_host":3,"OLED_host_markush":1,"OLED_blue_host_bipolar":1}',
 '{"S":2,"A":1}',
 '[{"type":"strengthen","priority":"medium","description":"File divisional on Markush sub-genera to block competitor design-arounds in KR market"},{"type":"extend_jurisdiction","priority":"high","description":"Extend EP4123456A1 to JP and KR for display manufacturer coverage"}]',
 NOW() - interval '15 days'),

('f0000001-0000-0000-0000-000000000002', 62.4, 65, 58, 42, 55, 38, 4, 3, 0, 0,
 '{"CN":2,"US":0,"JP":1}',
 '{"OLED_HTL":2,"OLED_device_structure":1,"OLED_fabrication_method":1}',
 '{"B":1,"C":2,"D":1}',
 '[{"type":"acquire","priority":"high","description":"Acquire or license externally developed HTL materials with higher Tg (>150°C) for blue OLED applications"},{"type":"abandon","priority":"medium","description":"Consider abandoning CN108864095B as it has expired and adds maintenance overhead"}]',
 NOW() - interval '15 days'),

('f0000001-0000-0000-0000-000000000003', 78.2, 80, 85, 72, 82, 25, 5, 4, 0, 0,
 '{"US":2,"EP":2,"KR":1}',
 '{"OLED_phosphorescent_emitter":1,"OLED_TADF_emitter":1,"OLED_ETL":1,"OLED_encapsulation":1,"OLED_host":1}',
 '{"S":1,"A":1,"B":1,"D":1}',
 '[{"type":"file_new","priority":"critical","description":"File new TADF emitter application covering deep-blue emission (<460nm) with ΔEST < 0.05eV"},{"type":"license_out","priority":"high","description":"Package US11832456B2 + US20230389421A1 for Samsung Display licensing negotiation Q3 2026"}]',
 NOW() - interval '15 days');

-- ── Portfolio Optimization Suggestions ────────────────────────────────────────
INSERT INTO portfolio_optimization_suggestions (portfolio_id, suggestion_type, priority, title, description, target_tech_domain, target_jurisdiction, estimated_impact, estimated_cost, rationale, status) VALUES
('f0000001-0000-0000-0000-000000000001', 'extend_jurisdiction', 'high', 'Extend EP4123456A1 to Japan and Korea',
 'The EP spirofluorene-triazine application should be extended to JP and KR jurisdictions through PCT national phase entry. Samsung and LG Display are the largest OLED panel manufacturers and have significant operations in both countries.',
 'H10K-085/60', 'JP', 85.0, 150000,
 '{"decision_matrix":{"competitive_blocking":90,"revenue_potential":80,"legal_strength":82},"risk_assessment":"Moderate - EP4123456A1 is still under examination, could face similar prior art challenges as the family"}',
 'pending'),
('f0000001-0000-0000-0000-000000000001', 'strengthen', 'medium', 'File divisional on Markush sub-genera',
 'CN116425750A has enormous Markush coverage (>50M compounds). File narrower divisionals on the most commercially promising sub-genera (dibenzofuran core with specific R-group combinations) to strengthen enforceability.',
 'C07D-307/91', 'CN', 72.0, 80000,
 '{"decision_matrix":{"competitive_blocking":78,"revenue_potential":65,"legal_strength":75},"risk_assessment":"Low - divisionals strengthen position without additional examination risk"}',
 'pending'),
('f0000001-0000-0000-0000-000000000002', 'acquire', 'high', 'Acquire high-Tg HTL material patents',
 'Current HTL portfolio lacks materials with Tg >150°C suitable for high-temperature OLED applications (automotive lighting). Target acquisition of external patents covering spiro-OMeTAD or similar high-Tg HTL structures.',
 'H10K-050/15', 'US', 78.0, 250000,
 '{"decision_matrix":{"competitive_blocking":82,"revenue_potential":72,"legal_strength":80},"risk_assessment":"Moderate - acquisition cost may be offset by expanded market access"}',
 'pending'),
('f0000001-0000-0000-0000-000000000003', 'file_new', 'critical', 'File deep-blue TADF emitter application',
 'File new patent application covering TADF emitters with emission <460nm, ΔEST <0.05eV, targeting the rapidly growing demand for true-blue OLED emitters for UHD displays.',
 'C09K-011/06', 'US', 95.0, 120000,
 '{"decision_matrix":{"competitive_blocking":95,"revenue_potential":98,"legal_strength":92},"risk_assessment":"Low - first-mover advantage in deep-blue TADF space, clear path to grant"}',
 'pending'),
('f0000001-0000-0000-0000-000000000003', 'license_out', 'high', 'Package Ir phosphorescent + TADF for Samsung license',
 'Bundle US11832456B2 (Ir phosphorescent) and US20230389421A1 (TADF) into a licensing package targeting Samsung Display. Combined value proposition for both phosphorescent and TADF display technologies.',
 'H10K-085/30', 'KR', 90.0, 50000,
 '{"decision_matrix":{"competitive_blocking":88,"revenue_potential":95,"legal_strength":90},"risk_assessment":"Low - Samsung is a known licensee of OLED material patents, established negotiation framework"}',
 'pending');

-- ══════════════════════════════════════════════════════════════════════════════
-- LIFECYCLE: ANNUITIES
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO patent_annuities (patent_id, year_number, due_date, grace_deadline, status, amount, currency) VALUES
-- CN115650927B - filed 2022, need annuity from year 3 (2025)
('d0000001-0000-0000-0000-000000000001', 3, '2025-03-18', '2025-09-18', 'paid', 900, 'CNY'),
('d0000001-0000-0000-0000-000000000001', 4, '2026-03-18', '2026-09-18', 'upcoming', 1200, 'CNY'),
('d0000001-0000-0000-0000-000000000001', 5, '2027-03-18', '2027-09-18', 'upcoming', 1500, 'CNY'),
-- CN116003389B - filed 2022, granted 2023
('d0000001-0000-0000-0000-000000000010', 3, '2025-06-20', '2025-12-20', 'paid', 900, 'CNY'),
('d0000001-0000-0000-0000-000000000010', 4, '2026-06-20', '2026-12-20', 'due', 1200, 'CNY'),
-- US11832456B2 - filed 2021, US maintenance at 3.5, 7.5, 11.5 years
('d0000001-0000-0000-0000-000000000003', 1, '2024-12-15', '2025-06-15', 'paid', 2000, 'USD'),
('d0000001-0000-0000-0000-000000000003', 2, '2027-12-15', '2028-06-15', 'upcoming', 3760, 'USD'),
-- US11456789B2 - filed 2022
('d0000001-0000-0000-0000-000000000011', 1, '2025-09-15', '2026-03-15', 'paid', 2000, 'USD'),
('d0000001-0000-0000-0000-000000000011', 2, '2028-09-15', '2029-03-15', 'upcoming', 3760, 'USD'),
-- EP3985742A1 - EP annual renewal from year 3
('d0000001-0000-0000-0000-000000000005', 3, '2024-09-03', '2025-03-03', 'paid', 470, 'EUR'),
('d0000001-0000-0000-0000-000000000005', 4, '2025-09-03', '2026-03-03', 'grace_period', 585, 'EUR'),
('d0000001-0000-0000-0000-000000000005', 5, '2026-09-03', '2027-03-03', 'upcoming', 810, 'EUR'),
-- JP7234567B2 - JP annual from year 1
('d0000001-0000-0000-0000-000000000006', 1, '2022-04-22', '2022-10-22', 'paid', 4300, 'JPY'),
('d0000001-0000-0000-0000-000000000006', 2, '2023-04-22', '2023-10-22', 'paid', 4300, 'JPY'),
('d0000001-0000-0000-0000-000000000006', 3, '2024-04-22', '2024-10-22', 'paid', 4300, 'JPY'),
('d0000001-0000-0000-0000-000000000006', 4, '2025-04-22', '2025-10-22', 'paid', 4300, 'JPY'),
('d0000001-0000-0000-0000-000000000006', 5, '2026-04-22', '2026-10-22', 'upcoming', 4300, 'JPY');

-- ══════════════════════════════════════════════════════════════════════════════
-- LIFECYCLE: DEADLINES
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO patent_deadlines (patent_id, deadline_type, title, description, due_date, original_due_date, status, priority, assignee_id) VALUES
-- Upcoming deadlines (within 30-90 days)
('d0000001-0000-0000-0000-000000000001', 'annuity_payment', 'CN115650927B - 维持费第4年',
 '第4年专利维持费缴纳，截止日2026年3月18日，宽限期至2026年9月18日',
 '2026-03-18 23:59:00+08', '2026-03-18 23:59:00+08', 'active', 'critical', 'b0000001-0000-0000-0000-000000000002'),
('d0000001-0000-0000-0000-000000000010', 'annuity_payment', 'CN116003389B - 维持费第4年',
 '第4年专利维持费缴纳，截止日2026年6月20日',
 '2026-06-20 23:59:00+08', '2026-06-20 23:59:00+08', 'active', 'high', 'b0000001-0000-0000-0000-000000000002'),
('d0000001-0000-0000-0000-000000000005', 'annuity_payment', 'EP3985742A1 - 4th renewal fee grace deadline',
 '4th year renewal grace period deadline. Late payment penalty applies.',
 '2026-03-03 23:59:00+01', '2025-09-03 23:59:00+01', 'active', 'critical', 'b0000001-0000-0000-0000-000000000001'),
('d0000001-0000-0000-0000-000000000006', 'annuity_payment', 'JP7234567B2 - 5th annual fee',
 '第5年年金納付期限 2026年4月22日',
 '2026-04-22 23:59:00+09', '2026-04-22 23:59:00+09', 'active', 'high', 'b0000001-0000-0000-0000-000000000002'),
('d0000001-0000-0000-0000-000000000002', 'office_action_response', 'CN116354946A - 审查意见答复',
 '中国专利局发出的审查意见通知书，需在4个月内答复',
 '2026-06-05 23:59:00+08', '2026-02-05 23:59:00+08', 'active', 'critical', 'b0000001-0000-0000-0000-000000000005'),
('d0000001-0000-0000-0000-000000000013', 'office_action_response', 'CN116425750A - 第一次审查意见答复',
 'Markush通式专利申请第一次审查意见，需论证创造性',
 '2026-07-12 23:59:00+08', '2026-03-12 23:59:00+08', 'active', 'high', 'b0000001-0000-0000-0000-000000000005'),
-- Future deadlines
('d0000001-0000-0000-0000-000000000003', 'maintenance_fee', 'US11832456B2 - 7.5 year maintenance fee',
 'USPTO 7.5 year maintenance fee due. Failure results in patent expiration.',
 '2028-06-28 23:59:00-05', '2028-06-28 23:59:00-05', 'active', 'medium', 'b0000001-0000-0000-0000-000000000001'),
('d0000001-0000-0000-0000-000000000012', 'examination_request', 'EP4123456A1 - Request for examination',
 'Must file examination request within 6 months of publication.',
 '2024-01-12 23:59:00+01', '2023-07-12 23:59:00+01', 'completed', 'medium', 'b0000001-0000-0000-0000-000000000001'),
-- Overdue/missed
('d0000001-0000-0000-0000-000000000008', 'annuity_payment', 'CN108864095B - Final annuity missed',
 '专利年费未缴，已失效。最后缴费期限2022年5月12日。',
 '2022-05-12 23:59:00+08', '2022-05-12 23:59:00+08', 'missed', 'low', NULL);

-- ══════════════════════════════════════════════════════════════════════════════
-- LIFECYCLE: EVENTS
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO patent_lifecycle_events (patent_id, event_type, event_date, title, description, before_state, after_state) VALUES
('d0000001-0000-0000-0000-000000000001', 'filing', '2022-03-18 10:30:00+08', '专利申请提交',
 '通过中国国家知识产权局电子申请系统提交发明专利申请，申请号CN202210271890.5',
 '{}', '{"status":"draft","application_number":"CN202210271890.5"}'),
('d0000001-0000-0000-0000-000000000001', 'publication', '2023-08-29 09:00:00+08', '专利申请公布',
 '经过18个月保密期，专利申请依法公布',
 '{"status":"pending"}', '{"status":"published","publication_number":"CN115650927A"}'),
('d0000001-0000-0000-0000-000000000001', 'examination_request', '2023-03-15 14:00:00+08', '请求实质审查',
 '在申请日起3年内提出实质审查请求',
 '{"status":"published"}', '{"status":"under_examination"}'),
('d0000001-0000-0000-0000-000000000001', 'office_action', '2023-11-10 16:30:00+08', '第一次审查意见通知书',
 '审查员认为权利要求1-8相对于对比文件D1-D3不具备创造性',
 '{"status":"under_examination"}', '{"status":"office_action_received","oa_count":1}'),
('d0000001-0000-0000-0000-000000000001', 'response_filed', '2023-12-20 11:00:00+08', '提交审查意见答复',
 '针对第一次审查意见的答复，修改了权利要求并进行了创造性争辩',
 '{"status":"office_action_received"}', '{"status":"response_filed"}'),
('d0000001-0000-0000-0000-000000000001', 'grant', '2024-01-05 09:00:00+08', '授予专利权',
 '专利局发出授权通知书，缴纳授权登记费后颁发专利证书',
 '{"status":"response_filed"}', '{"status":"granted","grant_date":"2024-01-05","patent_number":"CN115650927B"}'),
('d0000001-0000-0000-0000-000000000010', 'filing', '2022-06-20 09:00:00+08', '分案申请提交',
 '基于母案CN115650927B提交分案申请',
 '{}', '{"application_number":"CN202210700123.4","parent_application":"CN202210271890.5"}'),
('d0000001-0000-0000-0000-000000000010', 'grant', '2023-12-15 10:00:00+08', '分案专利授权',
 '分案申请获授权，专利号CN116003389B',
 '{"status":"under_examination"}', '{"status":"granted","patent_number":"CN116003389B"}'),
('d0000001-0000-0000-0000-000000000008', 'expiry', '2022-05-12 00:00:00+08', '专利失效',
 '因未按时缴纳年费，专利权终止',
 '{"status":"granted"}', '{"status":"expired","reason":"failure_to_pay_annuity"}'),
('d0000001-0000-0000-0000-000000000009', 'opposition', '2021-03-15 10:00:00+01', '第三方异议',
 '竞争对手提出异议，主张专利缺乏创造性',
 '{"status":"granted"}', '{"status":"under_opposition"}'),
('d0000001-0000-0000-0000-000000000009', 'invalidation', '2022-09-20 14:00:00+01', '专利撤销',
 '经异议程序，EPO决定撤销该专利',
 '{"status":"under_opposition"}', '{"status":"revoked","revocation_reason":"opposition_prior_art"}');

-- ══════════════════════════════════════════════════════════════════════════════
-- AUDIT LOGS (representative actions)
-- ══════════════════════════════════════════════════════════════════════════════
INSERT INTO audit_logs (user_id, organization_id, action, resource_type, resource_id, ip_address, user_agent, created_at) VALUES
('b0000001-0000-0000-0000-000000000001', 'a0000001-0000-0000-0000-000000000001', 'login', 'session', NULL, '192.168.99.1', 'Mozilla/5.0 Chrome', NOW()),
('b0000001-0000-0000-0000-000000000002', 'a0000001-0000-0000-0000-000000000001', 'view', 'patent', 'd0000001-0000-0000-0000-000000000001', '192.168.1.100', 'Mozilla/5.0 Chrome', NOW() - interval '1 day'),
('b0000001-0000-0000-0000-000000000003', 'a0000001-0000-0000-0000-000000000001', 'search', 'molecule', NULL, '10.0.1.50', 'Mozilla/5.0 Chrome', NOW() - interval '1 day'),
('b0000001-0000-0000-0000-000000000004', 'a0000001-0000-0000-0000-000000000001', 'export', 'portfolio', 'f0000001-0000-0000-0000-000000000001', '192.168.1.200', 'Mozilla/5.0 Chrome', NOW() - interval '2 days'),
('b0000001-0000-0000-0000-000000000005', 'a0000001-0000-0000-0000-000000000001', 'update', 'deadline', '', '203.0.113.45', 'Mozilla/5.0 Chrome', NOW() - interval '2 days'),
('b0000001-0000-0000-0000-000000000001', 'a0000001-0000-0000-0000-000000000001', 'create', 'report', NULL, '192.168.99.1', 'Mozilla/5.0 Chrome', NOW() - interval '3 days');

-- +migrate Down
-- Seed data removal (reverse order of inserts)
DELETE FROM audit_logs WHERE id IN (SELECT id FROM audit_logs ORDER BY created_at DESC LIMIT 6);
DELETE FROM patent_lifecycle_events WHERE patent_id IN (SELECT id FROM patents WHERE patent_number LIKE 'CN%' OR patent_number LIKE 'US%' OR patent_number LIKE 'EP%' OR patent_number LIKE 'JP%' OR patent_number LIKE 'KR%');
DELETE FROM patent_deadlines;
DELETE FROM patent_annuities;
DELETE FROM portfolio_optimization_suggestions;
DELETE FROM portfolio_health_scores;
DELETE FROM patent_valuations;
DELETE FROM portfolio_patents;
DELETE FROM portfolios;
DELETE FROM patent_molecule_relations;
DELETE FROM patent_claims;
DELETE FROM patent_inventors;
DELETE FROM patents;
DELETE FROM molecule_properties;
DELETE FROM molecules;
DELETE FROM user_roles;
DELETE FROM organization_members;
DELETE FROM users;
DELETE FROM organizations;