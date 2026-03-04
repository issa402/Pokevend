// ============================================================
// PokémonTool — Auth Routes
// POST /api/auth/register
// POST /api/auth/login
// POST /api/auth/logout
// ============================================================

const express    = require('express');
const router     = express.Router();
const authCtrl   = require('../controllers/authController');

// @desc    Register a new vendor account
// @access  Public
router.post('/register', authCtrl.register);

// @desc    Log in with email + password, returns JWT
// @access  Public
router.post('/login', authCtrl.login);

// @desc    Logout (client should discard the token; server logs it)
// @access  Public
router.post('/logout', authCtrl.logout);

module.exports = router;
