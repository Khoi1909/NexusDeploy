# Thesis Risk Register

**Project:** NexusDeploy Thesis  
**Last Updated:** 2025-11-03  
**Review Frequency:** Weekly (Every Monday)

---

## Risk Scoring

- **Probability:** LOW (1) | MEDIUM (2) | HIGH (3)
- **Impact:** LOW (1) | MEDIUM (2) | CRITICAL (3)
- **Risk Score:** Probability × Impact
- **Priority:** Score ≥ 6 = High Priority

---

## High Priority Risks (Score ≥ 6)

### RISK-001: Docker Networking Issues
**Probability:** HIGH (3) | **Impact:** CRITICAL (3) | **Score:** 9

**Description:**
Services cannot communicate via Docker network (DNS resolution fails, port conflicts)

**Mitigation:**
- Test on fresh VM 2 weeks before defense
- Document complete docker-compose setup procedure
- Have backup: Pre-recorded demo video
- Practice recovery: Rebuild network from scratch

**Contingency:**
If occurs during demo → Switch to backup video, explain architecture verbally

**Status:** OPEN  
**Owner:** DevOps

---

### RISK-002: GitHub OAuth Configuration Failure
**Probability:** MEDIUM (2) | **Impact:** CRITICAL (3) | **Score:** 6

**Description:**
OAuth redirect URL misconfigured, callback fails, or GitHub API down

**Mitigation:**
- Setup OAuth app 3 weeks early (Week 2)
- Test extensively on different networks
- Have backup: Dev login endpoint with pre-seeded JWT
- Document OAuth setup for advisor to verify

**Contingency:**
Use `/auth/dev-login` endpoint, skip OAuth, show auth code in slides

**Status:** OPEN  
**Owner:** Backend

---

### RISK-004: Time Overrun (Cannot Complete All Features)
**Probability:** HIGH (3) | **Impact:** MEDIUM (2) | **Score:** 6

**Description:**
Week 7-8 debugging takes longer than expected, AI integration not ready

**Mitigation:**
- Prioritize: Core CI/CD flow > AI feature
- Week 10 checkpoint: If behind schedule, cut AI feature
- Minimum viable demo: Login → Create Project → Build → Deploy
- Document: "AI analysis designed but not implemented"

**Contingency:**
Focus thesis on CI/CD architecture, show AI as "future work"

**Status:** OPEN  
**Owner:** PM

---

## Medium Priority Risks (Score 3-5)

### RISK-003: LLM API Rate Limit / Timeout
**Probability:** MEDIUM (2) | **Impact:** HIGH (2) | **Score:** 4

**Description:**
AI Service cannot call LLM API (rate limit, network timeout, API key expired)

**Mitigation:**
- Cache successful AI responses in Redis
- Pre-load 3-5 cached responses for demo
- Monitor API quota 1 week before defense
- Document: "Showing cached response for demo reliability"

**Contingency:**
Switch to cached response immediately, explain in presentation

**Status:** OPEN  
**Owner:** Backend (AI Service)

---

### RISK-006: Build Hanging/Timeout
**Probability:** MEDIUM (2) | **Impact:** MEDIUM (2) | **Score:** 4

**Description:**
Docker build for user app hangs, times out, or fails unexpectedly during demo

**Mitigation:**
- Use simple demo app (minimal dependencies)
- Pre-test demo repo build 20+ times
- Set build timeout to 3 minutes
- Have backup: Pre-built container ready

**Contingency:**
Deploy pre-built container, show logs from recorded build

**Status:** OPEN  
**Owner:** Backend (Runner Service)

---

### RISK-007: WebSocket Connection Drops
**Probability:** MEDIUM (2) | **Impact:** MEDIUM (2) | **Score:** 4

**Description:**
WebSocket connection fails during log streaming in demo

**Mitigation:**
- Implement auto-reconnect (Task 5.3)
- Test on slow network
- Show "Reconnecting..." UI indicator
- Have backup: Show logs via REST API polling

**Contingency:**
Refresh page (auto-reconnect), or switch to polling mode

**Status:** OPEN  
**Owner:** Frontend

---

### RISK-009: Next.js SSR Hydration Mismatch
**Probability:** MEDIUM (2) | **Impact:** MEDIUM (2) | **Score:** 4

**Description:**
Next.js hydration errors cause blank screens, console warnings, or content flickering during demo

**Mitigation:**
- SSR guards on all browser-specific code (`typeof window !== 'undefined'`)
- `'use client'` directive on all client components
- Test SSR build with `npm run build && npm start`
- Document SSR pitfalls in frontend docs

**Contingency:**
Refresh page, explain it's SSR issue, continue with backend demo if persists

**Status:** OPEN  
**Owner:** Frontend

---

### RISK-010: Next.js Build Fails in Docker
**Probability:** MEDIUM (2) | **Impact:** MEDIUM (2) | **Score:** 4

**Description:**
Production Docker build fails due to missing dependencies, build errors, or OOM

**Mitigation:**
- Multi-stage Dockerfile (deps → builder → runner)
- Test production build weekly: `docker build -t nexus-frontend ./frontend`
- Monitor build time (< 5 minutes target)
- Validation script checks Dockerfile syntax

**Contingency:**
Use dev server (`npm run dev`) instead of production build for demo

**Status:** OPEN  
**Owner:** DevOps

---

## Low Priority Risks (Score ≤ 2)

### RISK-005: Database Migration Errors
**Probability:** LOW (1) | **Impact:** HIGH (2) | **Score:** 2

**Description:**
Migration fails, database schema corrupted, data loss

**Mitigation:**
- Version control all migrations in Git
- Test migrations on fresh database before applying
- Backup database before each migration
- Document rollback procedure

**Contingency:**
Restore from backup, rebuild database from scratch (have init scripts)

**Status:** OPEN  
**Owner:** Backend (All services)

---

### RISK-008: Secrets Decryption Failure
**Probability:** LOW (1) | **Impact:** CRITICAL (3) | **Score:** 3

**Description:**
Master encryption key lost or secrets fail to decrypt

**Mitigation:**
- Backup master key in 3 locations (Git-ignored file, USB, advisor email)
- Test encryption/decryption 100 times before demo
- Document key in thesis appendix (for advisor only)
- Have backup: Fresh project without secrets

**Contingency:**
Create new project without secrets, show encryption code in slides

**Status:** OPEN  
**Owner:** Backend (Project Service)

---

## Risk Monitoring Schedule

### Weekly Review (Every Monday)
- [ ] Check GitHub OAuth still working
- [ ] Check LLM API quota remaining
- [ ] Check Docker Compose starts cleanly
- [ ] Check demo repo builds successfully
- [ ] Update risk scores based on progress
- [ ] Add new risks as identified

### 4 Weeks Before Defense (Week 9)
- [ ] Full demo rehearsal on fresh VM
- [ ] Test all backup scenarios
- [ ] Verify all contingency plans work
- [ ] Create backup video (10 minutes)

### 2 Weeks Before Defense (Week 11)
- [ ] Daily demo rehearsal
- [ ] Monitor all external APIs
- [ ] Freeze code (no new features)
- [ ] Create backup demo environment

### 1 Week Before Defense (Week 12)
- [ ] Final rehearsal with advisor
- [ ] Test on defense room equipment
- [ ] Print backup materials
- [ ] Prepare Q&A responses

---

## Risk Response Protocol

**When Risk Occurs:**
1. Assess impact (can demo continue?)
2. Execute contingency plan
3. Document incident
4. Update risk register

**Communication:**
- Critical risks → Inform advisor immediately
- Medium risks → Mention in weekly update
- Low risks → Document in project log

---

## Closed Risks

(None yet - project just starting)

---

**Next Review:** 2025-11-10 (Week 2 Monday)

