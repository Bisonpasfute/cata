package warlock

import (
	"strconv"
	"time"

	"github.com/wowsims/wotlk/sim/core"
	"github.com/wowsims/wotlk/sim/core/stats"
)

func (warlock *Warlock) registerCorruptionSpell() {
	actionID := core.ActionID{SpellID: 27216}
	baseCost := 0.14 * warlock.BaseMana

	warlock.Corruption = warlock.RegisterSpell(core.SpellConfig{
		ActionID:     actionID,
		SpellSchool:  core.SpellSchoolShadow,
		ResourceType: stats.Mana,
		BaseCost:     baseCost,
		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				Cost: baseCost * (1 - 0.02*float64(warlock.Talents.Suppression)),
				GCD:  core.GCDDefault,
			},
		},
		ApplyEffects: core.ApplyEffectFuncDirectDamage(core.SpellEffect{
			ProcMask:        core.ProcMaskSpellDamage,
			OutcomeApplier:  warlock.OutcomeFuncMagicHit(),
			OnSpellHitDealt: applyDotOnLanded(&warlock.CorruptionDot),
		}),
	})
	target := warlock.CurrentTarget
	spellCoefficient := 0.156 + ((0.12 * float64(warlock.Talents.EmpoweredCorruption)) / 6)

	warlock.CorruptionDot = core.NewDot(core.Dot{
		Spell: warlock.Corruption,
		Aura: target.RegisterAura(core.Aura{
			Label:    "Corruption-" + strconv.Itoa(int(warlock.Index)),
			ActionID: actionID,
		}),
		NumberOfTicks: 6,
		TickLength:    time.Second * 3,
		TickEffects: core.TickFuncSnapshot(target, core.SpellEffect{
			ProcMask:         core.ProcMaskPeriodicDamage,
			DamageMultiplier: 1 * (1 + 0.02*float64(warlock.Talents.ShadowMastery)) * (1 + 0.01*float64(warlock.Talents.Contagion)) * (1 + 0.01*float64(warlock.Talents.ImprovedCorruption)),
			ThreatMultiplier: 1 - 0.1*float64(warlock.Talents.ImprovedDrainSoul),
			BaseDamage:       core.BaseDamageConfigMagicNoRoll(1080/6, spellCoefficient),
			OutcomeApplier:   warlock.OutcomeFuncTick(),
			IsPeriodic:       true,
		}),
	})
}

func applyDotOnLanded(dot **core.Dot) func(sim *core.Simulation, spell *core.Spell, spellEffect *core.SpellEffect) {
	return func(sim *core.Simulation, spell *core.Spell, spellEffect *core.SpellEffect) {
		if spellEffect.Landed() {
			(*dot).Apply(sim)
		}
	}
}
