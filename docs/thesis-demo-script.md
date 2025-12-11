# Thesis Defense Demo Script

**Duration:** 15 minutes  
**Environment:** Localhost (all services pre-started)  
**Audience:** Advisor + 2-3 reviewers  
**Updated:** 2025-11-03

---

## Pre-Demo Checklist (30 minutes before)

- [ ] All services running: `docker-compose ps` (all healthy)
- [ ] Next.js dev server running: `http://localhost:3000` accessible
- [ ] Test user account: `testuser@thesis.demo` created
- [ ] Sample repo forked: `nexus-demo-app` (Node.js app)
- [ ] Backup video recorded (full flow, 10 minutes)
- [ ] Cached AI response ready (in case LLM API fails)
- [ ] Slides loaded on secondary screen

---

## Demo Flow (Primary Scenario)

### Part 1: Authentication (2 minutes)
1. Open browser: `http://localhost:3000`
2. Show login page (clean UI)
3. Click "Login with GitHub"
4. **BACKUP:** If OAuth fails, use pre-seeded JWT token via dev endpoint
5. Show dashboard (empty state initially)

### Part 2: Project Creation (3 minutes)
6. Click "+ New Project"
7. Fill form:
   - Name: "Demo App"
   - Repo: Select `nexus-demo-app` from GitHub list
   - Preset: Node.js
8. Click "Create Project"
9. Show project detail page
10. **Point out:** Webhook automatically configured on GitHub

### Part 3: Build Trigger (4 minutes)
11. Open terminal, show Git repo
12. Make small change: `echo "Demo" >> README.md`
13. Git commit & push
14. **Switch to UI:** Show build triggered (status: pending → running)
15. Show real-time log streaming (WebSocket)
16. **Wait for build to complete** (~2 minutes)
17. Show status: success
18. Show deployment info (subdomain URL)

### Part 4: Deployment Access (2 minutes)
19. Click subdomain link: `demo-app-abc123.khqi.io.vn`
20. Show deployed application running
21. **Explain:** Traefik auto-routing, SSL certificate

### Part 5: AI Analysis (3 minutes)
22. Trigger failed build (pre-prepared broken branch)
23. Git checkout broken-branch && git push
24. Wait for build failure
25. Click "🤖 Tell me why" button
26. **Show AI analysis:**
    - Error explanation
    - Suggested fix
27. **BACKUP:** If LLM API timeout, show cached response

### Part 6: Q&A Buffer (1 minute)
28. Open questions from reviewers

---

## Backup Scenarios

### Scenario A: OAuth Completely Broken
**Problem:** GitHub OAuth not responding  
**Solution:**
1. Use dev login endpoint: `POST /auth/dev-login`
2. Pre-seeded JWT: `test-jwt-token-12345`
3. Skip to dashboard (already logged in)
4. Continue from Part 2

### Scenario B: Build Hangs or Times Out
**Problem:** Docker build stuck  
**Solution:**
1. Stop demo, acknowledge issue
2. Play backup video (recorded successful build)
3. Show pre-deployed app
4. Continue to AI analysis

### Scenario C: LLM API Rate Limited
**Problem:** AI Service returns 429  
**Solution:**
1. Click "Tell me why"
2. Show loading state
3. After 5 seconds, switch to cached response
4. Explain: "Normally calls real API, showing cached for demo"

### Scenario D: Total System Failure
**Problem:** Docker Compose down, cannot recover  
**Solution:**
1. Switch to backup laptop (pre-started system)
2. OR switch to recorded video (full demo)
3. Walk through code instead of live demo

### Scenario E: Frontend Crashes (Next.js)
**Problem:** Next.js dev server crashes, hydration errors, or blank screen  
**Solution:**
1. Check browser console for errors
2. Refresh page (Fast Refresh may recover)
3. If persists → Show architecture slides, explain frontend components
4. Continue demo with backend-only: Show API calls via curl/Postman
5. Explain: "Frontend is Next.js 15 with SSR, showing cached state"

---

## Key Talking Points

**Architecture Strengths:**
- Microservices architecture (8 services)
- Event-driven CI/CD (Redis queue)
- Real-time updates (WebSocket)
- AI integration (unique feature)

**Acknowledged Limitations:**
- Single server deployment (not HA)
- Manual key management (production needs Vault)
- Limited load testing (5-10 concurrent builds)
- No distributed tracing (documented in thesis)

**Thesis Contribution:**
- Novel: AI-powered build failure analysis
- Demonstrates: Full-stack distributed systems knowledge
- Practical: Working end-to-end CI/CD platform

---

## Rehearsal Schedule

- [ ] Rehearsal 1: Week 12 Monday (full run, 20 minutes)
- [ ] Rehearsal 2: Week 12 Thursday (timed, 15 minutes)
- [ ] Rehearsal 3: Week 13 Monday (final polish, with advisor)

---

## Equipment Checklist

- [ ] Laptop (primary)
- [ ] Laptop (backup)
- [ ] HDMI cable + adapter
- [ ] USB drive (backup video + slides)
- [ ] Printed code snippets (in case projector fails)
- [ ] Water bottle

