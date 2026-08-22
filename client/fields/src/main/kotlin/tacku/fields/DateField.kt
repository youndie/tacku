package tacku.fields

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.KompotModifierNode
import io.github.youndie.kompot.form.FormCondition
import io.github.youndie.kompot.form.FormFieldDefinition
import io.github.youndie.kompot.form.ValidationRule
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.modules.SerializersModule
import kotlinx.serialization.modules.polymorphic
import kotlinx.serialization.modules.subclass

/**
 * A date, which the protocol's vocabulary does not have.
 *
 * The prototype set a due date with a text input, a mask and a server-side check, and the design
 * named the scenario that has no answer without a field of its own: people put a due date relative
 * to a week, not in ISO. "Next Friday" in a text box is arithmetic done in the head, and every
 * typo costs a round trip.
 *
 * **This is the deployment extending the vocabulary, which is the point of it existing at all.**
 * The names reach the profile as a branch of the hierarchy, so an ordinary JSON Schema library
 * accepts them and refuses anything not declared. Nothing about the mechanism is ours: it is what
 * §2.4 grew for exactly this, and this deployment appears to be its first user outside the toolkit.
 *
 * **The value stays a `text_value`.** A date on the wire is an ISO string, and inventing a value
 * type would extend a second hierarchy for nothing — the one that carries the actual data, where a
 * mistake costs the whole response rather than one node.
 */
@Serializable
@SerialName("date_field")
data class DateField(
    override val fieldId: String,
    override val rules: List<ValidationRule> = emptyList(),
    override val visibleIf: FormCondition? = null,
    override val triggersPatch: Boolean = false,
    /** The initial value, an ISO date, or empty for none. */
    val value: String = "",
    /** The earliest date offered, ISO, or empty for no bound. */
    val min: String = "",
    /** The latest date offered, ISO, or empty for no bound. */
    val max: String = "",
) : FormFieldDefinition

/**
 * What draws it.
 *
 * Separate from the definition for the same reason every other field is: the schema half travels to
 * anything that validates, the tree half only to something that renders.
 *
 * `displayFormat` is a finished pattern rather than a locale, because the server owns every string
 * on the screen and a client that formatted the date itself would be the one place where it did
 * not.
 */
@Serializable
@SerialName("date_input")
data class DateInput(
    override val id: String,
    override val modifiers: List<KompotModifierNode> = emptyList(),
    val fieldId: String,
    val label: String,
    val displayFormat: String = "d MMM yyyy",
    val placeholder: String = "",
    val hint: String = "",
) : KompotComponent

/**
 * How the two types join the vocabulary at runtime.
 *
 * The same names appear in three places and each is a different half of the same statement: this
 * module registers them with the serializer, the profile declares them for anything that validates,
 * and the registry maps one of them to something that draws. A name present in one and missing from
 * another is exactly the failure this extension exists to make impossible to write by accident, so
 * the trio is checked by a test rather than kept in step by hand.
 */
val tackuFieldsSerializersModule: SerializersModule =
    SerializersModule {
        polymorphic(KompotComponent::class) {
            subclass(DateInput::class, DateInput.serializer())
        }
        polymorphic(FormFieldDefinition::class) {
            subclass(DateField::class, DateField.serializer())
        }
    }
